package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/clappingmonkey/zuul-mcp/internal/config"
	"github.com/clappingmonkey/zuul-mcp/internal/server"
)

func TestServeStdio_StopsWhenClientStopsReading(t *testing.T) {
	oldTimeout, oldAbandoned := stdioShutdownTimeout, stdioAbandoned
	abandoned := make(chan (<-chan error), 1)
	stdioShutdownTimeout = 200 * time.Millisecond
	stdioAbandoned = func(done <-chan error) { abandoned <- done }
	t.Cleanup(func() { stdioShutdownTimeout, stdioAbandoned = oldTimeout, oldAbandoned })

	s := server.New(&config.Config{ZuulURL: "https://zuul.example.com"}, "test")
	inR, inW := io.Pipe()
	outR, outW := io.Pipe() // never read: every response write blocks
	// Unblock and join the abandoned Listen so it cannot touch mcp-go's
	// shared stdio session while a later test is using it.
	t.Cleanup(func() {
		inW.Close()
		outR.Close()
		select {
		case done := <-abandoned:
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("abandoned Listen did not return after its pipes were closed")
			}
		default:
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveStdio(ctx, s.MCPServer(), slog.New(slog.DiscardHandler), inR, outW) }()

	go func() {
		for i := 1; i <= 20; i++ {
			if _, err := fmt.Fprintf(inW, `{"jsonrpc":"2.0","id":%d,"method":"tools/list"}`+"\n", i); err != nil {
				return
			}
		}
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveStdio() = %v, want nil", err)
		}
		if len(abandoned) != 1 {
			t.Fatal("serveStdio() returned without hitting the shutdown timeout")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serveStdio() did not return after cancellation")
	}
}

func TestServeStdio_EndOfInput(t *testing.T) {
	s := server.New(&config.Config{ZuulURL: "https://zuul.example.com"}, "test")
	inR, inW := io.Pipe()
	inW.Close()
	err := serveStdio(context.Background(), s.MCPServer(), slog.New(slog.DiscardHandler), inR, io.Discard)
	if err != nil {
		t.Fatalf("serveStdio() = %v, want nil on EOF", err)
	}
}

func TestWarnDeprecated(t *testing.T) {
	for _, tc := range []struct {
		transport string
		wantWarn  bool
	}{
		{"stdio", false},
		{"http", false},
		{"sse", true},
	} {
		t.Run(tc.transport, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))
			warnDeprecated(logger, &config.Config{Transport: tc.transport})
			out := buf.String()
			got := strings.Contains(out, "level=WARN") && strings.Contains(out, "deprecated") && strings.Contains(out, "-transport=http")
			if got != tc.wantWarn {
				t.Errorf("warnDeprecated(%q) logged %q, want warning = %v", tc.transport, out, tc.wantWarn)
			}
		})
	}
}

func TestZuulHost(t *testing.T) {
	for in, want := range map[string]string{
		"https://zuul.example.com/secret/path": "zuul.example.com",
		"http://127.0.0.1:9000":                "127.0.0.1:9000",
		"::bad":                                "",
	} {
		if got := zuulHost(in); got != want {
			t.Errorf("zuulHost(%q) = %q, want %q", in, got, want)
		}
	}
}
