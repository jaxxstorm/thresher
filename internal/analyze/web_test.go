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

	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
)

type fakeWebRuntime struct {
	open func(context.Context) (*webEndpoint, error)
}

func (r fakeWebRuntime) Open(ctx context.Context) (*webEndpoint, error) {
	return r.open(ctx)
}

type fakeServeClient struct {
	serveConfig *ipn.ServeConfig
	setConfigs  []*ipn.ServeConfig
	status      *ipnstate.Status
	getErr      error
	setErr      error
	statusErr   error
}

func (c *fakeServeClient) GetServeConfig(context.Context) (*ipn.ServeConfig, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}
	return cloneServeConfig(c.serveConfig), nil
}

func (c *fakeServeClient) SetServeConfig(_ context.Context, config *ipn.ServeConfig) error {
	if c.setErr != nil {
		return c.setErr
	}
	next := cloneServeConfig(config)
	c.setConfigs = append(c.setConfigs, next)
	c.serveConfig = cloneServeConfig(next)
	return nil
}

func (c *fakeServeClient) StatusWithoutPeers(context.Context) (*ipnstate.Status, error) {
	if c.statusErr != nil {
		return nil, c.statusErr
	}
	return c.status, nil
}

func TestTailnetWebRuntimeConfiguresServeMountAndCleansUp(t *testing.T) {
	client := &fakeServeClient{
		status: &ipnstate.Status{Self: &ipnstate.PeerStatus{DNSName: "thresher.tail.ts.net."}},
		serveConfig: &ipn.ServeConfig{
			ETag: "etag-1",
			Web: map[ipn.HostPort]*ipn.WebServerConfig{
				"thresher.tail.ts.net:443": {
					Handlers: map[string]*ipn.HTTPHandler{
						"/existing/": {Text: "keep"},
					},
				},
			},
			TCP: map[uint16]*ipn.TCPPortHandler{
				443: {HTTPS: true},
			},
		},
	}

	runtime := tailnetWebRuntime{
		newLocalClient: func() serveConfigClient { return client },
	}

	endpoint, err := runtime.Open(context.Background())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if endpoint.baseURL != "https://thresher.tail.ts.net/thresher/" {
		t.Fatalf("unexpected ready url %q", endpoint.baseURL)
	}
	if endpoint.routePrefix != thresherServeMount {
		t.Fatalf("unexpected route prefix %q", endpoint.routePrefix)
	}
	if len(client.setConfigs) != 1 {
		t.Fatalf("expected one Serve config update, got %d", len(client.setConfigs))
	}

	cfg := client.setConfigs[0]
	if cfg.ETag != "etag-1" {
		t.Fatalf("expected Serve config ETag to be preserved, got %q", cfg.ETag)
	}
	handler := getServeMountHandler(cfg, "thresher.tail.ts.net", 443, thresherServeMount)
	if handler == nil {
		t.Fatal("expected Thresher Serve mount to be configured")
	}
	if !isThresherServeHandler(handler) {
		t.Fatalf("expected Thresher-owned handler, got %#v", handler)
	}
	if !strings.Contains(handler.Proxy, endpoint.listener.Addr().String()) {
		t.Fatalf("expected proxy target to include listener address, got %q", handler.Proxy)
	}
	if getServeMountHandler(cfg, "thresher.tail.ts.net", 443, "/existing/") == nil {
		t.Fatal("expected unrelated Serve mount to be preserved")
	}

	if err := endpoint.shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
	if len(client.setConfigs) != 2 {
		t.Fatalf("expected cleanup Serve config update, got %d total updates", len(client.setConfigs))
	}
	cleaned := client.setConfigs[1]
	if getServeMountHandler(cleaned, "thresher.tail.ts.net", 443, thresherServeMount) != nil {
		t.Fatal("expected Thresher Serve mount to be cleaned up")
	}
	if getServeMountHandler(cleaned, "thresher.tail.ts.net", 443, "/existing/") == nil {
		t.Fatal("expected unrelated Serve mount to survive cleanup")
	}
}

func TestTailnetWebRuntimeRejectsClaimedServeMount(t *testing.T) {
	client := &fakeServeClient{
		status: &ipnstate.Status{Self: &ipnstate.PeerStatus{DNSName: "thresher.tail.ts.net."}},
		serveConfig: &ipn.ServeConfig{
			Web: map[ipn.HostPort]*ipn.WebServerConfig{
				"thresher.tail.ts.net:443": {
					Handlers: map[string]*ipn.HTTPHandler{
						thresherServeMount: {Text: "owned by something else"},
					},
				},
			},
			TCP: map[uint16]*ipn.TCPPortHandler{
				443: {HTTPS: true},
			},
		},
	}

	runtime := tailnetWebRuntime{
		newLocalClient: func() serveConfigClient { return client },
	}

	_, err := runtime.Open(context.Background())
	if err == nil {
		t.Fatal("expected Serve mount collision error")
	}
	if !strings.Contains(err.Error(), "already claimed") {
		t.Fatalf("expected claimed-route error, got %v", err)
	}
	if len(client.setConfigs) != 0 {
		t.Fatalf("expected no Serve config updates on collision, got %d", len(client.setConfigs))
	}
}

func TestWebPresenterTailnetAccessDeniesAllRoutesWithoutCapability(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	presenter := newWebPresenterWithRuntime(fakeWebRuntime{
		open: func(context.Context) (*webEndpoint, error) {
			return &webEndpoint{
				listener:    listener,
				baseURL:     "http://" + listener.Addr().String() + thresherServeMount,
				routePrefix: thresherServeMount,
				wrap:        authorizeTailnetRequests,
				shutdown:    func(context.Context) error { return listener.Close() },
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
	for _, route := range []string{"", "snapshot", "events", "control/pause", "control/model"} {
		method := http.MethodGet
		body := io.Reader(nil)
		if strings.HasPrefix(route, "control/") {
			method = http.MethodPost
			body = strings.NewReader(`{"model":"gpt-4o","paused":true}`)
		}

		req, err := http.NewRequest(method, url+route, body)
		if err != nil {
			t.Fatalf("NewRequest(%s) error = %v", route, err)
		}
		if method == http.MethodPost {
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

func TestWebPresenterTailnetAccessUsesServePrefixAndControls(t *testing.T) {
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
				listener:    listener,
				baseURL:     "http://" + listener.Addr().String() + thresherServeMount,
				routePrefix: thresherServeMount,
				wrap:        authorizeTailnetRequests,
				shutdown:    func(context.Context) error { return listener.Close() },
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

	req, err := http.NewRequest(http.MethodGet, "http://"+listener.Addr().String()+"/snapshot", nil)
	if err != nil {
		t.Fatalf("NewRequest(unprefixed snapshot) error = %v", err)
	}
	addCapabilityHeader(req)
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Do(unprefixed snapshot) error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected unprefixed snapshot route to 404, got %d", resp.StatusCode)
	}

	req, err = http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("NewRequest(page) error = %v", err)
	}
	addCapabilityHeader(req)
	resp, err = httpClient.Do(req)
	if err != nil {
		t.Fatalf("Do(page) error = %v", err)
	}
	pageBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("ReadAll(page) error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected page ok, got %d", resp.StatusCode)
	}
	for _, needle := range []string{"fetch('snapshot')", "new EventSource('events')", "postJSON('control/model'"} {
		if !strings.Contains(string(pageBody), needle) {
			t.Fatalf("expected page to contain %q", needle)
		}
	}

	req, err = http.NewRequest(http.MethodGet, url+"snapshot", nil)
	if err != nil {
		t.Fatalf("NewRequest(snapshot) error = %v", err)
	}
	addCapabilityHeader(req)
	resp, err = httpClient.Do(req)
	if err != nil {
		t.Fatalf("Do(snapshot) error = %v", err)
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

	req, err = http.NewRequest(http.MethodPost, url+"control/model", strings.NewReader(`{"model":"claude-haiku-4-5"}`))
	if err != nil {
		t.Fatalf("NewRequest(model) error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	addCapabilityHeader(req)
	resp, err = httpClient.Do(req)
	if err != nil {
		t.Fatalf("Do(model) error = %v", err)
	}
	resp.Body.Close()
	if got := waitForSnapshot(t, state, func(snapshot SessionSnapshot) bool { return snapshot.Model == "claude-haiku-4-5" }).Model; got != "claude-haiku-4-5" {
		t.Fatalf("expected switched model, got %q", got)
	}

	req, err = http.NewRequest(http.MethodPost, url+"control/pause", strings.NewReader(`{"paused":true}`))
	if err != nil {
		t.Fatalf("NewRequest(pause) error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	addCapabilityHeader(req)
	resp, err = httpClient.Do(req)
	if err != nil {
		t.Fatalf("Do(pause) error = %v", err)
	}
	resp.Body.Close()
	if !waitForSnapshot(t, state, func(snapshot SessionSnapshot) bool { return snapshot.Paused }).Paused {
		t.Fatal("expected paused state")
	}

	req, err = http.NewRequest(http.MethodPost, url+"control/quit", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("NewRequest(quit) error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	addCapabilityHeader(req)
	resp, err = httpClient.Do(req)
	if err != nil {
		t.Fatalf("Do(quit) error = %v", err)
	}
	resp.Body.Close()

	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func addCapabilityHeader(req *http.Request) {
	req.Header.Set(tailscaleAppCapabilitiesHeader, `{"lbrlabs.com/cap/thresher":[true]}`)
}
