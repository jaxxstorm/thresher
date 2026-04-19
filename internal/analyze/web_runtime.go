package analyze

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
	"tailscale.com/tsnet"
)

type WebAccess string

const (
	WebAccessLocal   WebAccess = "local"
	WebAccessTailnet WebAccess = "tailnet"
)

const thresherCapability tailcfg.PeerCapability = "lbrlabs.com/cap/thresher"

type webRuntime interface {
	Open(context.Context) (*webEndpoint, error)
}

type webEndpoint struct {
	listener net.Listener
	baseURL  string
	wrap     func(http.Handler) http.Handler
	shutdown func(context.Context) error
}

type peerIdentityResolver interface {
	WhoIs(context.Context, string) (*apitype.WhoIsResponse, error)
}

type statusClient interface {
	StatusWithoutPeers(context.Context) (*ipnstate.Status, error)
}

type tailscaleLocalClient interface {
	peerIdentityResolver
	statusClient
}

type tsnetServer interface {
	Listen(network, addr string) (net.Listener, error)
	LocalClient() (tailscaleLocalClient, error)
	Close() error
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
		listener: listener,
		baseURL:  "http://" + listener.Addr().String(),
		wrap:     identityHTTPHandler,
		shutdown: func(context.Context) error { return nil },
	}, nil
}

type tailnetWebRuntime struct {
	config     Config
	newServer  func(Config) tsnetServer
	listenAddr string
}

func newWebRuntime(config Config) webRuntime {
	if config.WebAccess == WebAccessTailnet {
		return tailnetWebRuntime{
			config:    config,
			newServer: newTSNetServer,
		}
	}
	return localWebRuntime{addr: "127.0.0.1:0"}
}

func (r tailnetWebRuntime) Open(ctx context.Context) (*webEndpoint, error) {
	listenAddr := r.listenAddr
	if strings.TrimSpace(listenAddr) == "" {
		listenAddr = ":0"
	}

	server := r.newServer(r.config)
	listener, err := server.Listen("tcp", listenAddr)
	if err != nil {
		_ = server.Close()
		return nil, fmt.Errorf("starting analysis tailnet listener: %w", err)
	}

	client, err := server.LocalClient()
	if err != nil {
		_ = listener.Close()
		_ = server.Close()
		return nil, fmt.Errorf("starting analysis tailnet local client: %w", err)
	}

	baseURL := advertisedTailnetURL(ctx, client, listener, defaultTSNetHostname())
	return &webEndpoint{
		listener: listener,
		baseURL:  baseURL,
		wrap: func(next http.Handler) http.Handler {
			return authorizeTailnetRequests(client, next)
		},
		shutdown: func(context.Context) error {
			_ = listener.Close()
			return server.Close()
		},
	}, nil
}

type tsnetServerAdapter struct {
	*tsnet.Server
}

func (s *tsnetServerAdapter) LocalClient() (tailscaleLocalClient, error) {
	return s.Server.LocalClient()
}

func newTSNetServer(Config) tsnetServer {
	return &tsnetServerAdapter{Server: &tsnet.Server{Hostname: defaultTSNetHostname()}}
}

func defaultTSNetHostname() string {
	host, err := os.Hostname()
	if err != nil {
		host = ""
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return "thresher"
	}

	var b strings.Builder
	lastDash := false
	for _, r := range host {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-':
			if !lastDash && b.Len() > 0 {
				b.WriteRune(r)
				lastDash = true
			}
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}

	base := strings.Trim(b.String(), "-")
	if base == "" {
		base = "thresher"
	}
	if !strings.HasSuffix(base, "-thresher") {
		base += "-thresher"
	}
	if len(base) > 63 {
		base = strings.Trim(base[:63], "-")
	}
	if base == "" {
		return "thresher"
	}
	return base
}

func advertisedTailnetURL(ctx context.Context, client statusClient, listener net.Listener, fallbackHost string) string {
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return "http://" + listener.Addr().String()
	}

	if status, err := client.StatusWithoutPeers(ctx); err == nil && status != nil && status.Self != nil {
		if dnsName := strings.TrimSuffix(status.Self.DNSName, "."); dnsName != "" {
			host = dnsName
		}
	}
	if strings.TrimSpace(host) == "" {
		host = fallbackHost
	}
	return "http://" + net.JoinHostPort(host, port)
}

func authorizeTailnetRequests(resolver peerIdentityResolver, next http.Handler) http.Handler {
	if resolver == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "tailnet identity unavailable", http.StatusUnauthorized)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		who, err := resolver.WhoIs(r.Context(), r.RemoteAddr)
		if err != nil {
			http.Error(w, "tailnet identity required", http.StatusUnauthorized)
			return
		}
		if who == nil || !who.CapMap.HasCapability(thresherCapability) {
			http.Error(w, "missing required capability", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func identityHTTPHandler(next http.Handler) http.Handler {
	return next
}
