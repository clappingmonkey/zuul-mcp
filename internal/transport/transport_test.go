package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/clappingmonkey/zuul-mcp/internal/config"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func TestGuard(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name    string
		token   string
		allowed []string
		method  string
		origin  string
		auth    string
		preflt  bool
		want    int
	}{
		{name: "no origin, no token", method: http.MethodPost, want: http.StatusOK},
		{name: "foreign origin rejected", method: http.MethodPost, origin: "https://evil.example", want: http.StatusForbidden},
		{name: "rebinding origin rejected", method: http.MethodPost, origin: "http://evil.example:8080", want: http.StatusForbidden},
		{name: "loopback origin needs allowlisting", method: http.MethodPost, origin: "http://localhost:6274", want: http.StatusForbidden},
		{name: "allowlisted loopback origin", allowed: []string{"http://localhost:6274"}, method: http.MethodPost, origin: "http://localhost:6274", want: http.StatusOK},
		{name: "allowlisted origin", allowed: []string{"https://app.example"}, method: http.MethodPost, origin: "https://app.example", want: http.StatusOK},
		{name: "lookalike origin rejected", allowed: []string{"https://app.example"}, method: http.MethodPost, origin: "https://app.example.evil", want: http.StatusForbidden},
		{name: "null origin rejected", method: http.MethodPost, origin: "null", want: http.StatusForbidden},
		{name: "missing token", token: "s3cret", method: http.MethodPost, want: http.StatusUnauthorized},
		{name: "wrong token", token: "s3cret", method: http.MethodPost, auth: "Bearer nope", want: http.StatusUnauthorized},
		{name: "wrong scheme", token: "s3cret", method: http.MethodPost, auth: "Basic s3cret", want: http.StatusUnauthorized},
		{name: "right token", token: "s3cret", method: http.MethodPost, auth: "Bearer s3cret", want: http.StatusOK},
		{name: "case-insensitive scheme", token: "s3cret", method: http.MethodPost, auth: "bearer s3cret", want: http.StatusOK},
		{name: "extra spaces around token", token: "s3cret", method: http.MethodPost, auth: "Bearer   s3cret ", want: http.StatusOK},
		{name: "blank token", token: "s3cret", method: http.MethodPost, auth: "Bearer   ", want: http.StatusUnauthorized},
		{name: "token prefix", token: "s3cret", method: http.MethodPost, auth: "Bearer s3cre", want: http.StatusUnauthorized},
		{name: "preflight skips auth", token: "s3cret", allowed: []string{"https://app.example"}, method: http.MethodOptions, origin: "https://app.example", preflt: true, want: http.StatusOK},
		{name: "plain OPTIONS needs token", token: "s3cret", method: http.MethodOptions, want: http.StatusUnauthorized},
		{name: "preflight from foreign origin", token: "s3cret", method: http.MethodOptions, origin: "https://evil.example", preflt: true, want: http.StatusForbidden},
		{name: "foreign origin even with token", token: "s3cret", method: http.MethodPost, origin: "https://evil.example", auth: "Bearer s3cret", want: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/mcp", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			if tt.preflt {
				req.Header.Set("Access-Control-Request-Method", http.MethodPost)
			}
			rec := httptest.NewRecorder()
			Guard(ok, tt.token, tt.allowed).ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
			if rec.Code == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") == "" {
				t.Error("401 without WWW-Authenticate header")
			}
		})
	}
}

func newHandler(t *testing.T, cfg *config.Config) http.Handler {
	t.Helper()
	h, err := NewHandler(mcpserver.NewMCPServer("test", "0"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestNewHandler_StreamablePreflight(t *testing.T) {
	cfg := &config.Config{Transport: "http", HTTPAuthToken: "s3cret", HTTPAllowedOrigins: []string{"https://app.example"}}
	h := newHandler(t, cfg)

	req := httptest.NewRequest(http.MethodOptions, StreamableHTTPPath, nil)
	req.Header.Set("Origin", "https://app.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "authorization, content-type, mcp-protocol-version")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	allowHeaders := strings.ToLower(rec.Header().Get("Access-Control-Allow-Headers"))
	for _, want := range []string{"authorization", "content-type", "mcp-protocol-version", "mcp-session-id"} {
		if !strings.Contains(allowHeaders, want) {
			t.Errorf("Access-Control-Allow-Headers %q missing %s", allowHeaders, want)
		}
	}
}

func TestNewHandler_StreamableRequiresToken(t *testing.T) {
	h := newHandler(t, &config.Config{Transport: "http", HTTPAuthToken: "s3cret"})
	req := httptest.NewRequest(http.MethodPost, StreamableHTTPPath, strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestNewHandler_SSENoWildcardCORS(t *testing.T) {
	ts := httptest.NewServer(newHandler(t, &config.Config{Transport: "sse"}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/sse", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("SSE sent Access-Control-Allow-Origin %q", got)
	}
}

// A reverse proxy on the same machine connects over loopback but forwards
// the public Host header.
func TestNewHandler_ProxyHostHeader(t *testing.T) {
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`
	for _, tt := range []struct {
		name  string
		token string
		want  int
	}{
		{name: "no token keeps rebinding protection", want: http.StatusForbidden},
		{name: "token replaces rebinding protection", token: "s3cret", want: http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(newHandler(t, &config.Config{Transport: "http", HTTPAuthToken: tt.token}))
			defer ts.Close()
			req, _ := http.NewRequest(http.MethodPost, ts.URL+StreamableHTTPPath, strings.NewReader(initialize))
			req.Host = "zuul-mcp.example.com"
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tt.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.want)
			}
		})
	}
}

func TestNewHandler_RejectsStdio(t *testing.T) {
	if _, err := NewHandler(mcpserver.NewMCPServer("test", "0"), &config.Config{Transport: "stdio"}); err == nil {
		t.Error("expected error for stdio")
	}
}

func TestNewServerTimeouts(t *testing.T) {
	srv := NewServer("127.0.0.1:0", http.NotFoundHandler())
	if srv.ReadHeaderTimeout == 0 || srv.IdleTimeout == 0 {
		t.Errorf("timeouts not set: %+v", srv)
	}
}
