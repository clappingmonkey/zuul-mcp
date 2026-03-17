package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
		check   func(*Config) bool
	}{
		{
			name:    "missing ZUUL_URL",
			envVars: map[string]string{},
			wantErr: true,
		},
		{
			name: "valid minimal config",
			envVars: map[string]string{
				"ZUUL_URL": "https://zuul.example.com",
			},
			wantErr: false,
			check: func(c *Config) bool {
				return c.ZuulURL == "https://zuul.example.com" &&
					c.Transport == "stdio" &&
					c.HTTPPort == "8080"
			},
		},
		{
			name: "full config",
			envVars: map[string]string{
				"ZUUL_URL":            "https://zuul.example.com",
				"ZUUL_DEFAULT_TENANT": "my-tenant",
				"ZUUL_AUTH_TOKEN":     "secret-token",
				"ZUUL_TRANSPORT":      "http",
				"ZUUL_HTTP_PORT":      "9090",
			},
			wantErr: false,
			check: func(c *Config) bool {
				return c.ZuulURL == "https://zuul.example.com" &&
					c.DefaultTenant == "my-tenant" &&
					c.AuthToken == "secret-token" &&
					c.Transport == "http" &&
					c.HTTPPort == "9090" &&
					c.HasAuth()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.check != nil && !tt.check(cfg) {
				t.Errorf("Load() config validation failed: %+v", cfg)
			}
		})
	}
}

func TestHasAuth(t *testing.T) {
	cfg := &Config{}
	if cfg.HasAuth() {
		t.Error("HasAuth() should return false for empty token")
	}

	cfg.AuthToken = "some-token"
	if !cfg.HasAuth() {
		t.Error("HasAuth() should return true for non-empty token")
	}
}
