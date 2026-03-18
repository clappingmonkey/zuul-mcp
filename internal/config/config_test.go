package config

import (
	"os"
	"path/filepath"
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

func TestLoadEnvFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		presetEnv   map[string]string
		wantErr     bool
		errContains string
		check       func() bool
	}{
		{
			name: "basic key-value pairs",
			content: `ZUUL_URL=https://zuul.example.com
ZUUL_AUTH_TOKEN=secret123`,
			check: func() bool {
				return os.Getenv("ZUUL_URL") == "https://zuul.example.com" &&
					os.Getenv("ZUUL_AUTH_TOKEN") == "secret123"
			},
		},
		{
			name: "comments and empty lines",
			content: `# This is a comment
ZUUL_URL=https://zuul.example.com

# Another comment
ZUUL_DEFAULT_TENANT=my-tenant
`,
			check: func() bool {
				return os.Getenv("ZUUL_URL") == "https://zuul.example.com" &&
					os.Getenv("ZUUL_DEFAULT_TENANT") == "my-tenant"
			},
		},
		{
			name:    "double quoted values",
			content: `ZUUL_URL="https://zuul.example.com"`,
			check: func() bool {
				return os.Getenv("ZUUL_URL") == "https://zuul.example.com"
			},
		},
		{
			name:    "single quoted values",
			content: `ZUUL_URL='https://zuul.example.com'`,
			check: func() bool {
				return os.Getenv("ZUUL_URL") == "https://zuul.example.com"
			},
		},
		{
			name:    "value containing equals sign",
			content: `ZUUL_AUTH_TOKEN=abc=def=ghi`,
			check: func() bool {
				return os.Getenv("ZUUL_AUTH_TOKEN") == "abc=def=ghi"
			},
		},
		{
			name:    "whitespace trimming",
			content: `  ZUUL_URL  =  https://zuul.example.com  `,
			check: func() bool {
				return os.Getenv("ZUUL_URL") == "https://zuul.example.com"
			},
		},
		{
			name: "existing env var takes precedence",
			content: `ZUUL_URL=https://from-file.example.com
ZUUL_AUTH_TOKEN=file-token`,
			presetEnv: map[string]string{
				"ZUUL_URL": "https://from-env.example.com",
			},
			check: func() bool {
				return os.Getenv("ZUUL_URL") == "https://from-env.example.com" &&
					os.Getenv("ZUUL_AUTH_TOKEN") == "file-token"
			},
		},
		{
			name:        "missing equals sign",
			content:     `INVALID_LINE`,
			wantErr:     true,
			errContains: "missing '='",
		},
		{
			name:        "empty key",
			content:     `=value`,
			wantErr:     true,
			errContains: "empty key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set preset environment variables
			for k, v := range tt.presetEnv {
				os.Setenv(k, v)
			}

			// Create temp file
			tmpDir := t.TempDir()
			envFile := filepath.Join(tmpDir, ".env")
			if err := os.WriteFile(envFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			err := LoadEnvFile(envFile)

			if tt.wantErr {
				if err == nil {
					t.Error("LoadEnvFile() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("LoadEnvFile() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("LoadEnvFile() unexpected error: %v", err)
				return
			}

			if tt.check != nil && !tt.check() {
				t.Error("LoadEnvFile() check failed")
			}
		})
	}
}

func TestLoadEnvFile_FileNotFound(t *testing.T) {
	os.Clearenv()
	err := LoadEnvFile("/nonexistent/path/.env")
	if err == nil {
		t.Error("LoadEnvFile() expected error for non-existent file")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
