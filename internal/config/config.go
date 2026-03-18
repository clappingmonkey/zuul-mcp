// Package config provides configuration management for the Zuul MCP server.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
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

// LoadEnvFile loads environment variables from a .env file.
// Format: KEY=value (one per line, # for comments, empty lines ignored).
// Existing environment variables take precedence over values in the file.
func LoadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first '=' only (value may contain '=')
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid format at line %d: missing '='", lineNum)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return fmt.Errorf("invalid format at line %d: empty key", lineNum)
		}

		// Strip surrounding quotes if present (both single and double)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// Only set if not already defined in environment (existing env vars take precedence)
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("failed to set environment variable %s: %w", key, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading env file: %w", err)
	}

	return nil
}
