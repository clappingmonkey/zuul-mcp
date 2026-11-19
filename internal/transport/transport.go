// Package transport hardens the HTTP and SSE MCP transports: it validates
// the Origin header, optionally requires a bearer token, applies a matching
// CORS policy, and builds an http.Server with sane timeouts.
package transport

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/clappingmonkey/zuul-mcp/internal/config"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// StreamableHTTPPath is where the streamable HTTP transport is served.
const StreamableHTTPPath = "/mcp"

// NewHandler returns the guarded HTTP handler for cfg.Transport ("http" or
// "sse").
func NewHandler(s *mcpserver.MCPServer, cfg *config.Config) (http.Handler, error) {
	// mcp-go only emits CORS headers for exact matches of these origins. With
	// no configured origins we still enable CORS with an entry that can never
	// match, because the SSE transport otherwise answers with
	// Access-Control-Allow-Origin: *.
	origins := cfg.HTTPAllowedOrigins
	if len(origins) == 0 {
		origins = []string{""}
	}
	cors := []mcpserver.CORSOption{
		mcpserver.WithCORSAllowedOrigins(origins...),
		func(c *mcpserver.CORSConfig) {
			c.AllowedHeaders = []string{"Content-Type", "Authorization", "Mcp-Session-Id", "Mcp-Protocol-Version", "Last-Event-ID"}
		},
	}

	// mcp-go rejects loopback connections whose Host header is not loopback
	// (DNS rebinding protection). That also rejects a TLS reverse proxy that
	// forwards the public Host. When a bearer token is required, a rebinding
	// page cannot authenticate anyway, so the token takes over that role.
	tokenRequired := cfg.HTTPAuthToken != ""

	var handler http.Handler
	switch cfg.Transport {
	case "http":
		mux := http.NewServeMux()
		mux.Handle(StreamableHTTPPath, mcpserver.NewStreamableHTTPServer(s,
			mcpserver.WithStreamableHTTPCORS(cors...),
			mcpserver.WithDisableLocalhostProtection(tokenRequired),
		))
		handler = mux
	case "sse":
		handler = mcpserver.NewSSEServer(s,
			mcpserver.WithSSECORS(cors...),
			mcpserver.WithSSEDisableLocalhostProtection(tokenRequired),
		)
	default:
		return nil, fmt.Errorf("transport %q is not served over HTTP", cfg.Transport)
	}
	return Guard(handler, cfg.HTTPAuthToken, cfg.HTTPAllowedOrigins), nil
}

// Guard wraps next with Origin validation and, when token is non-empty,
// bearer-token authentication.
//
// The MCP specification requires servers to validate the Origin header to
// prevent DNS rebinding and cross-site attacks. Requests without an Origin
// (non-browser clients) are allowed. Browser requests are allowed only from
// allowedOrigins, the same list that drives the CORS headers, so an origin
// that passes here can also read the response. Everything else gets 403.
func Guard(next http.Handler, token string, allowedOrigins []string) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	tokenSum := sha256.Sum256([]byte(token))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && !allowed[origin] {
			http.Error(w, "Forbidden: origin not allowed", http.StatusForbidden)
			return
		}
		// CORS preflights never carry credentials; only those from allowed
		// origins get this far, and the MCP handler answers them.
		if token != "" && !isPreflight(r) {
			got, ok := bearerToken(r)
			gotSum := sha256.Sum256([]byte(got))
			match := subtle.ConstantTimeCompare(gotSum[:], tokenSum[:]) == 1
			if !ok || !match {
				w.Header().Set("WWW-Authenticate", `Bearer realm="zuul-mcp"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// NewServer returns an http.Server for addr with timeouts that protect
// against slow-header clients. No write timeout is set because MCP responses
// may be long-lived event streams.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}

// Shutdown timing: requests get drainTimeout to finish on their own, then
// their contexts are cancelled (ending long-lived event streams); the server
// gives up after shutdownTimeout and closes remaining connections.
var (
	drainTimeout    = 2 * time.Second
	shutdownTimeout = 10 * time.Second
)

// Serve runs srv on ln until ctx is cancelled, then shuts it down gracefully,
// force-closing connections still open after shutdownTimeout. It returns nil
// once a requested shutdown completes and an error only if serving fails.
func Serve(ctx context.Context, srv *http.Server, ln net.Listener) error {
	reqCtx, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	srv.BaseContext = func(net.Listener) context.Context { return reqCtx }

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	shutdownErr := make(chan error, 1)
	go func() { shutdownErr <- srv.Shutdown(shutdownCtx) }()

	var err error
	select {
	case err = <-shutdownErr:
	case <-time.After(drainTimeout):
		cancelRequests()
		err = <-shutdownErr
	}
	if err != nil {
		// Requested shutdown is not a failure: force-close what is left.
		slog.Warn("closing connections that did not finish in time", "timeout", shutdownTimeout, "error", err)
		_ = srv.Close()
	}
	return nil
}

func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}

func bearerToken(r *http.Request) (string, bool) {
	scheme, value, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	value = strings.TrimSpace(value)
	if !ok || !strings.EqualFold(scheme, "Bearer") || value == "" {
		return "", false
	}
	return value, true
}
