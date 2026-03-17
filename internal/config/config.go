// Package config provides configuration management for the Zuul MCP server.
package config

import (
	"errors"
	"os"
)

// Config holds the configuration for the Zuul MCP server.
type Config struct {
	// ZuulURL is the base URL of the Zuul instance (required).
	ZuulURL string

	// DefaultTenant is the default tenant to use if not specified in requests.
	DefaultTenant string

	// AuthToken is the JWT bearer token for authenticated endpoints.
	AuthToken string

	// Transport specifies the MCP transport mode: "stdio" or "http".
	Transport string

	// HTTPPort is the port for HTTP transport mode.
	HTTPPort string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		ZuulURL:       os.Getenv("ZUUL_URL"),
		DefaultTenant: os.Getenv("ZUUL_DEFAULT_TENANT"),
		AuthToken:     os.Getenv("ZUUL_AUTH_TOKEN"),
		Transport:     os.Getenv("ZUUL_TRANSPORT"),
		HTTPPort:      os.Getenv("ZUUL_HTTP_PORT"),
	}

	// Validate required fields
	if cfg.ZuulURL == "" {
		return nil, errors.New("ZUUL_URL environment variable is required")
	}

	// Set defaults
	if cfg.Transport == "" {
		cfg.Transport = "stdio"
	}
	if cfg.HTTPPort == "" {
		cfg.HTTPPort = "8080"
	}

	return cfg, nil
}

// HasAuth returns true if authentication is configured.
func (c *Config) HasAuth() bool {
	return c.AuthToken != ""
}
