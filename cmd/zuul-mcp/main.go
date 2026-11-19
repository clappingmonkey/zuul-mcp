// Package main provides the entry point for the Zuul MCP server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/clappingmonkey/zuul-mcp/internal/config"
	"github.com/clappingmonkey/zuul-mcp/internal/server"
	"github.com/clappingmonkey/zuul-mcp/internal/transport"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Logs go to stderr: on the stdio transport stdout carries the protocol.
	slog.SetDefault(newLogger(slog.LevelInfo))
	if err := run(); err != nil {
		slog.Error("zuul-mcp failed", "error", err)
		os.Exit(1)
	}
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

func run() error {
	envFile := flag.String("env-file", "", "Path to .env file for configuration")
	transportMode := flag.String("transport", "", "Transport mode: stdio (default), http, or sse")
	host := flag.String("host", "", "HTTP/SSE bind address (default: 127.0.0.1)")
	port := flag.String("port", "", "HTTP/SSE server port (default: 8080)")
	readOnly := flag.Bool("read-only", false, "Hide tools that change Zuul state (also ZUUL_READ_ONLY=true)")
	logLevel := flag.String("log-level", "", "Log level: debug, info (default), warn or error")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	resolveVersion()
	if *showVersion {
		fmt.Printf("zuul-mcp %s (%s) built on %s\n", version, commit, date)
		return nil
	}

	if *envFile != "" {
		if err := config.LoadEnvFile(*envFile); err != nil {
			return fmt.Errorf("loading env file: %w", err)
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Command line flags override the environment; validate the result.
	if *transportMode != "" {
		cfg.Transport = *transportMode
	}
	if *host != "" {
		cfg.HTTPHost = *host
	}
	if *port != "" {
		cfg.HTTPPort = *port
	}
	if *readOnly {
		cfg.ReadOnly = true
	}
	if *logLevel != "" {
		cfg.LogLevel = *logLevel
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	logger := newLogger(cfg.Level())
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		stop() // restore default handling: a second signal terminates at once
	}()

	s := server.New(cfg, version)
	slog.Info("starting zuul-mcp", "version", version, "transport", cfg.Transport, "zuul_host", zuulHost(cfg.ZuulURL))
	if reason := server.WriteToolsDisabledReason(cfg); reason != "" {
		slog.Info("tools that change Zuul state are disabled", "reason", reason)
	}

	if cfg.Transport == "stdio" {
		return serveStdio(ctx, s.MCPServer(), logger, os.Stdin, os.Stdout)
	}

	handler, err := transport.NewHandler(s.MCPServer(), cfg)
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	bindHost := strings.TrimSuffix(strings.TrimPrefix(cfg.HTTPHost, "["), "]")
	addr := net.JoinHostPort(bindHost, cfg.HTTPPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}
	if !config.IsLoopback(cfg.HTTPHost) {
		slog.Warn("listening on a non-loopback address; clients must send the ZUUL_HTTP_AUTH_TOKEN bearer token and TLS should terminate in front of this server", "addr", ln.Addr().String())
	}
	slog.Info("listening", "addr", ln.Addr().String())

	srv := transport.NewServer(addr, handler)
	srv.ErrorLog = slog.NewLogLogger(logger.Handler(), slog.LevelWarn)
	if err := transport.Serve(ctx, srv, ln); err != nil {
		return err
	}
	slog.Info("stopped")
	return nil
}

// stdioShutdownTimeout bounds how long a signalled stdio server waits for
// in-flight responses (e.g. writes blocked on a client that stopped reading).
var stdioShutdownTimeout = 5 * time.Second

// stdioAbandoned receives the result channel of a Listen call that
// serveStdio stopped waiting for; tests use it to join that goroutine.
var stdioAbandoned = func(<-chan error) {}

func serveStdio(ctx context.Context, s *mcpserver.MCPServer, logger *slog.Logger, in io.Reader, out io.Writer) error {
	stdio := mcpserver.NewStdioServer(s)
	stdio.SetErrorLogger(slog.NewLogLogger(logger.Handler(), slog.LevelError))
	done := make(chan error, 1)
	go func() { done <- stdio.Listen(ctx, in, out) }()

	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		select {
		case err = <-done:
		case <-time.After(stdioShutdownTimeout):
			slog.Warn("stopping without waiting for in-flight responses", "timeout", stdioShutdownTimeout)
			stdioAbandoned(done)
			return nil
		}
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("stdio server: %w", err)
	}
	slog.Info("stopped")
	return nil
}

// zuulHost returns only the host of the Zuul URL for logging: the path may
// carry deployment details that do not belong in logs.
func zuulHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

// resolveVersion fills in version information from the Go build info when
// the binary was not stamped by the release build (e.g. `go install`).
func resolveVersion() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = strings.TrimPrefix(info.Main.Version, "v")
	}
	for _, s := range info.Settings {
		switch {
		case s.Key == "vcs.revision" && commit == "unknown":
			commit = s.Value
		case s.Key == "vcs.time" && date == "unknown":
			date = s.Value
		}
	}
}
