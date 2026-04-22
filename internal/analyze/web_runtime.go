package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"slices"
	"strconv"
	"strings"

	"tailscale.com/client/local"
	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
)

type WebAccess string

const (
	WebAccessLocal   WebAccess = "local"
	WebAccessTailnet WebAccess = "tailnet"
)

const (
	thresherServeMount             = "/thresher/"
	tailscaleAppCapabilitiesHeader = "Tailscale-App-Capabilities"
)

const thresherCapability tailcfg.PeerCapability = "lbrlabs.com/cap/thresher"

type webRuntime interface {
	Open(context.Context) (*webEndpoint, error)
}

type webEndpoint struct {
	listener    net.Listener
	baseURL     string
	routePrefix string
	wrap        func(http.Handler) http.Handler
	shutdown    func(context.Context) error
}

type statusClient interface {
	StatusWithoutPeers(context.Context) (*ipnstate.Status, error)
}

type serveConfigClient interface {
	statusClient
	GetServeConfig(context.Context) (*ipn.ServeConfig, error)
	SetServeConfig(context.Context, *ipn.ServeConfig) error
}

type localWebRuntime struct {
	addr string
}

func (r localWebRuntime) Open(context.Context) (*webEndpoint, error) {
	addr := r.addr
	if strings.TrimSpace(addr) == "" {
		addr = "127.0.0.1:0"
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("starting analysis web listener: %w", err)
	}
	return &webEndpoint{
		listener:    listener,
		baseURL:     "http://" + listener.Addr().String(),
		routePrefix: "/",
		wrap:        identityHTTPHandler,
		shutdown:    func(context.Context) error { return listener.Close() },
	}, nil
}

type tailnetWebRuntime struct {
	addr           string
	newLocalClient func() serveConfigClient
}

func newWebRuntime(config Config) webRuntime {
	if config.WebAccess == WebAccessTailnet {
		return tailnetWebRuntime{
			addr:           "127.0.0.1:0",
			newLocalClient: func() serveConfigClient { return &local.Client{} },
		}
	}
	return localWebRuntime{addr: "127.0.0.1:0"}
}

func (r tailnetWebRuntime) Open(ctx context.Context) (*webEndpoint, error) {
	addr := r.addr
	if strings.TrimSpace(addr) == "" {
		addr = "127.0.0.1:0"
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("starting analysis localhost listener: %w", err)
	}

	client := r.newLocalClient()
	if client == nil {
		_ = listener.Close()
		return nil, fmt.Errorf("starting analysis tailnet access: tailscale local client unavailable")
	}

	baseURL, cleanup, err := configureServeAccess(ctx, client, listener.Addr().String())
	if err != nil {
		_ = listener.Close()
		return nil, err
	}

	return &webEndpoint{
		listener:    listener,
		baseURL:     baseURL,
		routePrefix: "/",
		wrap:        authorizeTailnetRequests,
		shutdown: func(ctx context.Context) error {
			return errors.Join(listener.Close(), cleanup(ctx))
		},
	}, nil
}

func configureServeAccess(ctx context.Context, client serveConfigClient, targetAddr string) (string, func(context.Context) error, error) {
	host, err := advertisedTailnetHost(ctx, client)
	if err != nil {
		return "", nil, err
	}

	current, err := client.GetServeConfig(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("reading Tailscale Serve config: %w", err)
	}
	next := cloneServeConfig(current)

	if err := claimServeMount(next, host, targetAddr); err != nil {
		return "", nil, err
	}
	if err := client.SetServeConfig(ctx, next); err != nil {
		return "", nil, fmt.Errorf("updating Tailscale Serve config: %w", err)
	}

	baseURL := "https://" + host + thresherServeMount
	cleanup := func(ctx context.Context) error {
		current, err := client.GetServeConfig(ctx)
		if err != nil {
			return fmt.Errorf("reading Tailscale Serve config for cleanup: %w", err)
		}
		if current == nil {
			return nil
		}
		handler := getServeMountHandler(current, host, 443, thresherServeMount)
		if !isThresherServeHandler(handler) {
			return nil
		}

		next := cloneServeConfig(current)
		next.RemoveWebHandler(host, 443, []string{thresherServeMount}, false)
		if err := client.SetServeConfig(ctx, next); err != nil {
			return fmt.Errorf("cleaning up Tailscale Serve config: %w", err)
		}
		return nil
	}

	return baseURL, cleanup, nil
}

func advertisedTailnetHost(ctx context.Context, client statusClient) (string, error) {
	status, err := client.StatusWithoutPeers(ctx)
	if err != nil {
		return "", fmt.Errorf("reading Tailscale status: %w", err)
	}
	if status == nil || status.Self == nil {
		return "", fmt.Errorf("reading Tailscale status: local device unavailable")
	}
	host := strings.TrimSuffix(strings.TrimSpace(status.Self.DNSName), ".")
	if host == "" {
		return "", fmt.Errorf("reading Tailscale status: local device has no MagicDNS name")
	}
	return host, nil
}

func cloneServeConfig(config *ipn.ServeConfig) *ipn.ServeConfig {
	if config == nil {
		return &ipn.ServeConfig{}
	}
	return config.Clone()
}

func claimServeMount(config *ipn.ServeConfig, host string, targetAddr string) error {
	if config == nil {
		config = &ipn.ServeConfig{}
	}

	handler := getServeMountHandler(config, host, 443, thresherServeMount)
	if handler != nil && !isThresherServeHandler(handler) {
		return fmt.Errorf("analysis tailnet route %s is already claimed on %s; remove or move the existing Tailscale Serve handler before retrying", thresherServeMount, host)
	}

	config.SetWebHandler(&ipn.HTTPHandler{
		Proxy:         "http://" + targetAddr,
		AcceptAppCaps: []tailcfg.PeerCapability{thresherCapability},
	}, host, 443, thresherServeMount, true, "")
	return nil
}

func getServeMountHandler(config *ipn.ServeConfig, host string, port uint16, mount string) *ipn.HTTPHandler {
	if config == nil || config.Web == nil {
		return nil
	}

	hp := ipn.HostPort(net.JoinHostPort(host, strconv.Itoa(int(port))))
	web := config.Web[hp]
	if web == nil || web.Handlers == nil {
		return nil
	}
	return web.Handlers[mount]
}

func isThresherServeHandler(handler *ipn.HTTPHandler) bool {
	if handler == nil {
		return false
	}
	if !slices.Equal(handler.AcceptAppCaps, []tailcfg.PeerCapability{thresherCapability}) {
		return false
	}
	return isLoopbackProxy(handler.Proxy)
}

func isLoopbackProxy(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}

	if strings.Contains(raw, "://") {
		parsed, err := neturl.Parse(raw)
		if err != nil {
			return false
		}
		host := parsed.Hostname()
		return host == "127.0.0.1" || host == "localhost"
	}

	host, _, err := net.SplitHostPort(raw)
	if err != nil {
		return false
	}
	return host == "127.0.0.1" || host == "localhost"
}

func authorizeTailnetRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimSpace(r.Header.Get(tailscaleAppCapabilitiesHeader))
		if raw == "" {
			http.Error(w, "missing required capability", http.StatusForbidden)
			return
		}

		var caps map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &caps); err != nil {
			http.Error(w, "invalid forwarded capability header", http.StatusForbidden)
			return
		}
		if _, ok := caps[string(thresherCapability)]; !ok {
			http.Error(w, "missing required capability", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func identityHTTPHandler(next http.Handler) http.Handler {
	return next
}
