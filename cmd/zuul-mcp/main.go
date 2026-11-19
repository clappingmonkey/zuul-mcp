// Package main provides the entry point for the Zuul MCP server.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

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
	// Parse command line flags
	envFile := flag.String("env-file", "", "Path to .env file for configuration")
	transportMode := flag.String("transport", "", "Transport mode: stdio (default), http, or sse")
	host := flag.String("host", "", "HTTP/SSE bind address (default: 127.0.0.1)")
	port := flag.String("port", "", "HTTP/SSE server port (default: 8080)")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("zuul-mcp %s (%s) built on %s\n", version, commit, date)
		os.Exit(0)
	}

	// Load environment variables from file if specified
	if *envFile != "" {
		if err := config.LoadEnvFile(*envFile); err != nil {
			log.Fatalf("Failed to load env file: %v", err)
		}
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override config with command line flags, then validate the result.
	if *transportMode != "" {
		cfg.Transport = *transportMode
	}
	if *host != "" {
		cfg.HTTPHost = *host
	}
	if *port != "" {
		cfg.HTTPPort = *port
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create the Zuul MCP server
	s := server.New(cfg)

	if cfg.Transport == "stdio" {
		log.Println("Starting Zuul MCP server with stdio transport...")
		if err := mcpserver.ServeStdio(s.MCPServer()); err != nil {
			log.Fatalf("Server error: %v", err)
		}
		return
	}

	handler, err := transport.NewHandler(s.MCPServer(), cfg)
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	bindHost := strings.TrimSuffix(strings.TrimPrefix(cfg.HTTPHost, "["), "]")
	addr := net.JoinHostPort(bindHost, cfg.HTTPPort)
	if !config.IsLoopback(cfg.HTTPHost) {
		log.Printf("WARNING: listening on non-loopback address %s; clients must send the ZUUL_HTTP_AUTH_TOKEN bearer token. Put TLS in front of this server.", addr)
	}
	log.Printf("Starting Zuul MCP server with %s transport on %s...", cfg.Transport, addr)
	srv := transport.NewServer(addr, handler)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
