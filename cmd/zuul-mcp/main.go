// Package main provides the entry point for the Zuul MCP server.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/clappingmonkey/zuul-mcp/internal/config"
	"github.com/clappingmonkey/zuul-mcp/internal/server"
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
	transport := flag.String("transport", "", "Transport mode: stdio (default) or http/sse")
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

	// Override config with command line flags
	if *transport != "" {
		cfg.Transport = *transport
	}
	if *port != "" {
		cfg.HTTPPort = *port
	}

	// Create the Zuul MCP server
	s := server.New(cfg)

	// Start the server based on transport mode
	switch cfg.Transport {
	case "stdio", "":
		log.Println("Starting Zuul MCP server with stdio transport...")
		if err := mcpserver.ServeStdio(s.MCPServer()); err != nil {
			log.Fatalf("Server error: %v", err)
		}

	case "http":
		addr := ":" + cfg.HTTPPort
		log.Printf("Starting Zuul MCP server with HTTP transport on %s...\n", addr)
		httpServer := mcpserver.NewStreamableHTTPServer(s.MCPServer())
		if err := httpServer.Start(addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}

	case "sse":
		addr := ":" + cfg.HTTPPort
		log.Printf("Starting Zuul MCP server with SSE transport on %s...\n", addr)
		sseServer := mcpserver.NewSSEServer(s.MCPServer())
		if err := sseServer.Start(addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}

	default:
		log.Fatalf("Unknown transport mode: %s (use 'stdio', 'http', or 'sse')", cfg.Transport)
	}
}
