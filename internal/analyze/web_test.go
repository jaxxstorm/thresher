package analyze

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
)

type fakeWebRuntime struct {
	open func(context.Context) (*webEndpoint, error)
}

func (r fakeWebRuntime) Open(ctx context.Context) (*webEndpoint, error) {
	return r.open(ctx)
}

type fakeTailnetClient struct {
	who        *apitype.WhoIsResponse
	whoErr     error
	status     *ipnstate.Status
	statusErr  error
	remoteAddr []string
}

func (c *fakeTailnetClient) WhoIs(_ context.Context, remoteAddr string) (*apitype.WhoIsResponse, error) {
	c.remoteAddr = append(c.remoteAddr, remoteAddr)
	return c.who, c.whoErr
}

func (c *fakeTailnetClient) StatusWithoutPeers(context.Context) (*ipnstate.Status, error) {
	return c.status, c.statusErr
}

type fakeTSNetServer struct {
	listener       net.Listener
	client         tailscaleLocalClient
	listenNetwork  string
	listenAddr     string
	localClientErr error
	closed         bool
}

func (s *fakeTSNetServer) Listen(network, addr string) (net.Listener, error) {
	s.listenNetwork = network
	s.listenAddr = addr
	return s.listener, nil
}

func (s *fakeTSNetServer) LocalClient() (tailscaleLocalClient, error) {
	if s.localClientErr != nil {
		return nil, s.localClientErr
	}
	return s.client, nil
}

func (s *fakeTSNetServer) Close() error {
	s.closed = true
	return nil
}

func TestTailnetWebRuntimeAdvertisesDNSNameAndClosesServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	client := &fakeTailnetClient{
		status: &ipnstate.Status{Self: &ipnstate.PeerStatus{DNSName: "thresher.tail.ts.net."}},
	}
	server := &fakeTSNetServer{listener: listener, client: client}
	runtime := tailnetWebRuntime{
		config: Config{WebAccess: WebAccessTailnet},
		newServer: func(Config) tsnetServer {
			return server
		},
	}

	endpoint, err := runtime.Open(context.Background())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	if endpoint.baseURL != "http://thresher.tail.ts.net:"+port {
		t.Fatalf("unexpected tailnet url %q", endpoint.baseURL)
	}
	if server.listenNetwork != "tcp" || server.listenAddr != ":0" {
		t.Fatalf("unexpected tsnet listen args network=%q addr=%q", server.listenNetwork, server.listenAddr)
	}

	if err := endpoint.shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
	if !server.closed {
		t.Fatal("expected tsnet server to close")
	}
}

func TestWebPresenterTailnetAccessDeniesAllRoutesWithoutCapability(t *testing.T) {
	client := &fakeTailnetClient{who: &apitype.WhoIsResponse{CapMap: tailcfg.PeerCapMap{}}}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	presenter := newWebPresenterWithRuntime(fakeWebRuntime{
		open: func(context.Context) (*webEndpoint, error) {
			return &webEndpoint{
				listener: listener,
				baseURL:  "http://" + listener.Addr().String(),
				wrap: func(next http.Handler) http.Handler {
					return authorizeTailnetRequests(client, next)
				},
				shutdown: func(context.Context) error { return nil },
			}, nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- presenter.Run(ctx, NewStateStore(Config{Model: "gpt-4o"}), func(runCtx context.Context) error {
			<-runCtx.Done()
			return nil
		})
	}()

	url := <-presenter.Ready()
	httpClient := &http.Client{Timeout: 2 * time.Second}
	for _, route := range []string{"/", "/snapshot", "/events", "/control/pause", "/control/model"} {
		req, err := http.NewRequest(http.MethodGet, url+route, nil)
		if err != nil {
			t.Fatalf("NewRequest(%s) error = %v", route, err)
		}
		if strings.HasPrefix(route, "/control/") {
			req.Method = http.MethodPost
			req.Body = io.NopCloser(strings.NewReader(`{"model":"gpt-4o","paused":true}`))
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			t.Fatalf("Do(%s) error = %v", route, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected forbidden for %s, got %d", route, resp.StatusCode)
		}
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestWebPresenterTailnetAccessAllowsSnapshotControlsAndQuit(t *testing.T) {
	capMap := tailcfg.PeerCapMap{thresherCapability: nil}
	client := &fakeTailnetClient{who: &apitype.WhoIsResponse{CapMap: capMap}}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	state := NewStateStore(Config{Endpoint: "http://ai", Model: "gpt-4o"})
	state.Update(func(snapshot *SessionSnapshot) {
		snapshot.Models = []string{"gpt-4o", "claude-haiku-4-5"}
	})

	presenter := newWebPresenterWithRuntime(fakeWebRuntime{
		open: func(context.Context) (*webEndpoint, error) {
			return &webEndpoint{
				listener: listener,
				baseURL:  "http://" + listener.Addr().String(),
				wrap: func(next http.Handler) http.Handler {
					return authorizeTailnetRequests(client, next)
				},
				shutdown: func(context.Context) error { return nil },
			}, nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- presenter.Run(ctx, state, func(runCtx context.Context) error {
			<-runCtx.Done()
			return nil
		})
	}()

	url := <-presenter.Ready()
	httpClient := &http.Client{Timeout: 2 * time.Second}

	resp, err := httpClient.Get(url + "/snapshot")
	if err != nil {
		t.Fatalf("Get(snapshot) error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected snapshot ok, got %d", resp.StatusCode)
	}
	var snapshot SessionSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		t.Fatalf("Decode(snapshot) error = %v", err)
	}
	if snapshot.Model != "gpt-4o" {
		t.Fatalf("unexpected snapshot %#v", snapshot)
	}

	resp, err = httpClient.Post(url+"/control/model", "application/json", strings.NewReader(`{"model":"claude-haiku-4-5"}`))
	if err != nil {
		t.Fatalf("Post(model) error = %v", err)
	}
	resp.Body.Close()
	if got := waitForSnapshot(t, state, func(state SessionSnapshot) bool { return state.Model == "claude-haiku-4-5" }).Model; got != "claude-haiku-4-5" {
		t.Fatalf("expected switched model, got %q", got)
	}

	resp, err = httpClient.Post(url+"/control/pause", "application/json", strings.NewReader(`{"paused":true}`))
	if err != nil {
		t.Fatalf("Post(pause) error = %v", err)
	}
	resp.Body.Close()
	if !waitForSnapshot(t, state, func(state SessionSnapshot) bool { return state.Paused }).Paused {
		t.Fatal("expected paused state")
	}

	resp, err = httpClient.Post(url+"/control/quit", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("Post(quit) error = %v", err)
	}
	resp.Body.Close()

	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	_, err = httpClient.Get(url + "/snapshot")
	if err == nil {
		t.Fatal("expected wrapped server shutdown after quit")
	}
}
