package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var configEnvVars = []string{
	"ZUUL_URL",
	"ZUUL_DEFAULT_TENANT",
	"ZUUL_AUTH_TOKEN",
	"ZUUL_TRANSPORT",
	"ZUUL_HTTP_PORT",
	"ZUUL_HTTP_HOST",
	"ZUUL_HTTP_AUTH_TOKEN",
	"ZUUL_HTTP_ALLOWED_ORIGINS",
	"ZUUL_ALLOW_INSECURE_HTTP",
	"ZUUL_READ_ONLY",
	"ZUUL_LOG_LEVEL",
	"ZUUL_AUTH_TOKEN_FILE",
	"ZUUL_HTTP_AUTH_TOKEN_FILE",
	"ENV_FILE_TEST_VAR",
}

// isolateEnv unsets every variable the config reads; t.Setenv restores the
// originals when the test ends.
func isolateEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for _, k := range configEnvVars {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr string
		check   func(*Config) bool
	}{
		{
			// Load only parses; Validate rejects the missing URL after any
			// command-line overrides are applied.
			name:    "missing ZUUL_URL is left to Validate",
			envVars: map[string]string{},
			check:   func(c *Config) bool { return c.ZuulURL == "" && c.Validate() != nil },
		},
		{
			name: "invalid port is left to Validate",
			envVars: map[string]string{
				"ZUUL_URL":       "https://zuul.example.com",
				"ZUUL_HTTP_PORT": "bad",
			},
			check: func(c *Config) bool { return c.HTTPPort == "bad" },
		},
		{
			name: "invalid allowed origin",
			envVars: map[string]string{
				"ZUUL_URL":                  "https://zuul.example.com",
				"ZUUL_HTTP_ALLOWED_ORIGINS": "https://app.example/path",
			},
			wantErr: "ZUUL_HTTP_ALLOWED_ORIGINS",
		},
		{
			name: "valid minimal config",
			envVars: map[string]string{
				"ZUUL_URL": "https://zuul.example.com",
			},
			check: func(c *Config) bool {
				return c.ZuulURL == "https://zuul.example.com" &&
					c.Transport == "stdio" &&
					c.HTTPPort == "8080" &&
					c.HTTPHost == "127.0.0.1" &&
					!c.AllowInsecureHTTP
			},
		},
		{
			name: "full config",
			envVars: map[string]string{
				"ZUUL_URL":                  "https://zuul.example.com",
				"ZUUL_DEFAULT_TENANT":       "my-tenant",
				"ZUUL_AUTH_TOKEN":           "secret-token",
				"ZUUL_TRANSPORT":            "http",
				"ZUUL_HTTP_PORT":            "9090",
				"ZUUL_HTTP_HOST":            "0.0.0.0",
				"ZUUL_HTTP_AUTH_TOKEN":      "client-token",
				"ZUUL_HTTP_ALLOWED_ORIGINS": " https://a.example , HTTPS://B.Example:443/ ,",
			},
			check: func(c *Config) bool {
				return c.ZuulURL == "https://zuul.example.com" &&
					c.DefaultTenant == "my-tenant" &&
					c.AuthToken == "secret-token" &&
					c.Transport == "http" &&
					c.HTTPPort == "9090" &&
					c.HTTPHost == "0.0.0.0" &&
					c.HTTPAuthToken == "client-token" &&
					len(c.HTTPAllowedOrigins) == 2 &&
					c.HTTPAllowedOrigins[0] == "https://a.example" &&
					c.HTTPAllowedOrigins[1] == "https://b.example" &&
					c.HasAuth()
			},
		},
		{
			name: "invalid ZUUL_ALLOW_INSECURE_HTTP",
			envVars: map[string]string{
				"ZUUL_URL":                 "https://zuul.example.com",
				"ZUUL_ALLOW_INSECURE_HTTP": "maybe",
			},
			wantErr: "ZUUL_ALLOW_INSECURE_HTTP",
		},
		{
			name: "insecure override accepted",
			envVars: map[string]string{
				"ZUUL_URL":                 "http://zuul.internal",
				"ZUUL_AUTH_TOKEN":          "secret-token",
				"ZUUL_ALLOW_INSECURE_HTTP": "true",
			},
			check: func(c *Config) bool { return c.AllowInsecureHTTP },
		},
		{
			name: "read-only mode",
			envVars: map[string]string{
				"ZUUL_URL":        "https://zuul.example.com",
				"ZUUL_AUTH_TOKEN": "secret-token",
				"ZUUL_READ_ONLY":  "true",
			},
			check: func(c *Config) bool { return c.ReadOnly && c.HasAuth() },
		},
		{
			name: "invalid ZUUL_READ_ONLY",
			envVars: map[string]string{
				"ZUUL_URL":       "https://zuul.example.com",
				"ZUUL_READ_ONLY": "yes please",
			},
			wantErr: "ZUUL_READ_ONLY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateEnv(t, tt.envVars)

			cfg, err := Load()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load() error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if tt.check != nil && !tt.check(cfg) {
				t.Errorf("Load() config validation failed: %+v", cfg)
			}
		})
	}
}

const strongToken = "0123456789abcdef0123456789abcdef" // gitleaks:allow (test fixture)

func TestValidate(t *testing.T) {
	valid := func() Config {
		return Config{
			ZuulURL:   "https://zuul.example.com",
			Transport: "stdio",
			HTTPHost:  "127.0.0.1",
			HTTPPort:  "8080",
		}
	}
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{name: "valid", mutate: func(*Config) {}},
		{name: "missing URL", mutate: func(c *Config) { c.ZuulURL = "" }, wantErr: "ZUUL_URL"},
		{name: "relative URL", mutate: func(c *Config) { c.ZuulURL = "zuul.example.com" }, wantErr: "absolute"},
		{name: "empty host", mutate: func(c *Config) { c.ZuulURL = "http://:8080" }, wantErr: "absolute"},
		{name: "bad URL port", mutate: func(c *Config) { c.ZuulURL = "http://127.0.0.1:99999" }, wantErr: "port"},
		{name: "query in URL", mutate: func(c *Config) { c.ZuulURL = "https://zuul.example.com/?a=b" }, wantErr: "query"},
		{name: "fragment in URL", mutate: func(c *Config) { c.ZuulURL = "https://zuul.example.com/#x" }, wantErr: "fragment"},
		{name: "query secret is not echoed", mutate: func(c *Config) { c.ZuulURL = "https://zuul.example.com/?token=hunter2" }, wantErr: "query"},
		{name: "bad scheme with secret path is not echoed", mutate: func(c *Config) { c.ZuulURL = "ftp://zuul.example.com/hunter2" }, wantErr: "http"},
		{name: "path prefix allowed", mutate: func(c *Config) { c.ZuulURL = "https://ci.example.com/zuul/" }},
		{
			name: "userinfo error does not echo the password",
			mutate: func(c *Config) {
				c.ZuulURL = "https://user:hunter2@zuul.example.com"
			},
			wantErr: "credentials",
		},
		{
			name: "lookalike localhost is not loopback",
			mutate: func(c *Config) {
				c.ZuulURL = "http://localhost.evil.example"
				c.AuthToken = "secret"
			},
			wantErr: "plain http",
		},
		{
			name: "uppercase scheme",
			mutate: func(c *Config) {
				c.ZuulURL = "HTTP://zuul.example.com"
				c.AuthToken = "secret"
			},
			wantErr: "plain http",
		},
		{
			name: "token over http to IPv6 loopback",
			mutate: func(c *Config) {
				c.ZuulURL = "http://[::1]:9000"
				c.AuthToken = "secret"
			},
		},
		{name: "ftp scheme", mutate: func(c *Config) { c.ZuulURL = "ftp://zuul.example.com" }, wantErr: "http"},
		{name: "userinfo in URL", mutate: func(c *Config) { c.ZuulURL = "https://u:p@zuul.example.com" }, wantErr: "credentials"},
		{name: "http without token", mutate: func(c *Config) { c.ZuulURL = "http://zuul.example.com" }},
		{
			name: "token over http to remote host",
			mutate: func(c *Config) {
				c.ZuulURL = "http://zuul.example.com"
				c.AuthToken = "secret"
			},
			wantErr: "plain http",
		},
		{
			name: "token over http to localhost",
			mutate: func(c *Config) {
				c.ZuulURL = "http://localhost:9000"
				c.AuthToken = "secret"
			},
		},
		{
			name: "token over http with override",
			mutate: func(c *Config) {
				c.ZuulURL = "http://zuul.example.com"
				c.AuthToken = "secret"
				c.AllowInsecureHTTP = true
			},
		},
		{name: "port zero", mutate: func(c *Config) { c.HTTPPort = "0" }, wantErr: "port"},
		{name: "port too large", mutate: func(c *Config) { c.HTTPPort = "70000" }, wantErr: "port"},
		{name: "port not a number", mutate: func(c *Config) { c.HTTPPort = "http" }, wantErr: "port"},
		{name: "unknown transport", mutate: func(c *Config) { c.Transport = "websocket" }, wantErr: "transport"},
		{
			name: "http on all interfaces without client token",
			mutate: func(c *Config) {
				c.Transport = "http"
				c.HTTPHost = "0.0.0.0"
			},
			wantErr: "ZUUL_HTTP_AUTH_TOKEN",
		},
		{
			name: "sse on empty host without client token",
			mutate: func(c *Config) {
				c.Transport = "sse"
				c.HTTPHost = ""
			},
			wantErr: "ZUUL_HTTP_AUTH_TOKEN",
		},
		{
			name: "http on all interfaces with client token",
			mutate: func(c *Config) {
				c.Transport = "http"
				c.HTTPHost = "0.0.0.0"
				c.HTTPAuthToken = strongToken
			},
		},
		{
			name: "short client token",
			mutate: func(c *Config) {
				c.Transport = "http"
				c.HTTPAuthToken = "1"
			},
			wantErr: "at least 32 characters",
		},
		{
			name: "client token with surrounding whitespace",
			mutate: func(c *Config) {
				c.Transport = "http"
				c.HTTPAuthToken = " " + strongToken
			},
			wantErr: "at least 32 characters",
		},
		{
			name: "stdio ignores host",
			mutate: func(c *Config) {
				c.HTTPHost = "0.0.0.0"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want error containing %q", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), "hunter2") {
				t.Errorf("error leaks the URL password: %v", err)
			}
		})
	}
}

func TestCanonicalOrigin(t *testing.T) {
	for in, want := range map[string]string{
		"https://app.example":           "https://app.example",
		"HTTPS://App.Example/":          "https://app.example",
		"https://app.example:443":       "https://app.example",
		"http://app.example:80":         "http://app.example",
		"http://localhost:6274":         "http://localhost:6274",
		"http://[::1]:6274":             "http://[::1]:6274",
		"https://app.example:8443/":     "https://app.example:8443",
		"http://[0:0:0:0:0:0:0:1]:6274": "http://[::1]:6274",
		"http://app.example:0080":       "http://app.example",
		"http://app.example:08080":      "http://app.example:8080",
		"http://127.0.0.1:6274":         "http://127.0.0.1:6274",
		"http://v1.example":             "http://v1.example",
	} {
		got, err := CanonicalOrigin(in)
		if err != nil || got != want {
			t.Errorf("CanonicalOrigin(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"app.example", "null", "*", "ftp://app.example", "https://app.example/path", "https://app.example?x=1", "https://u@app.example", "https://app.example:99999", "https://café.example", "http://[fe80::1%25en0]", "http://[::ffff:127.0.0.1]:6274", "http://[::ffff:7f00:1]:6274", "http://127.000.000.001:6274", "http://0x7f.1", "http://127.1"} {
		if got, err := CanonicalOrigin(bad); err == nil {
			t.Errorf("CanonicalOrigin(%q) = %q, want error", bad, got)
		}
	}
	_, err := CanonicalOrigin("https://user:s3cr3tpw@app.example")
	if err == nil || strings.Contains(err.Error(), "s3cr3tpw") {
		t.Errorf("error must not echo credentials: %v", err)
	}
}

func TestIsLoopback(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost":              true,
		"127.0.0.1":              true,
		"127.1.2.3":              true,
		"::1":                    true,
		"[::1]":                  true,
		"LOCALHOST":              true,
		"::ffff:127.0.0.1":       true,
		"localhost.":             false,
		"localhost.evil.example": false,
		"127.0.0.1.nip.io":       false,
		"0.0.0.0":                false,
		"":                       false,
		"10.0.0.1":               false,
		"example.com":            false,
	} {
		if got := IsLoopback(host); got != want {
			t.Errorf("IsLoopback(%q) = %v, want %v", host, got, want)
		}
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
			errContains: "variable names",
		},
		{
			name:        "invalid key",
			content:     `ZUUL URL=https://zuul.example.com`,
			wantErr:     true,
			errContains: "line 1",
		},
		{
			name:        "key starting with a digit",
			content:     "# c\n1ZUUL=x",
			wantErr:     true,
			errContains: "line 2",
		},
		{
			name:      "set but empty env var takes precedence",
			content:   `ZUUL_AUTH_TOKEN=file-token`,
			presetEnv: map[string]string{"ZUUL_AUTH_TOKEN": ""},
			check: func() bool {
				v, ok := os.LookupEnv("ZUUL_AUTH_TOKEN")
				return ok && v == ""
			},
		},
		{
			name:    "export prefix and BOM",
			content: "\ufeffexport ZUUL_URL=https://zuul.example.com\nexport\tZUUL_DEFAULT_TENANT=t",
			check: func() bool {
				return os.Getenv("ZUUL_URL") == "https://zuul.example.com" && os.Getenv("ZUUL_DEFAULT_TENANT") == "t"
			},
		},
		{
			name:    "variable named export",
			content: `exportX=1`,
			check: func() bool {
				return os.Getenv("exportX") == "1"
			},
		},
		{
			name: "inline comments",
			content: `ZUUL_URL=https://zuul.example.com # production
ZUUL_AUTH_TOKEN=abc#def
ZUUL_DEFAULT_TENANT="my tenant" # quoted
ZUUL_HTTP_HOST='a # b'`,
			check: func() bool {
				return os.Getenv("ZUUL_URL") == "https://zuul.example.com" &&
					os.Getenv("ZUUL_AUTH_TOKEN") == "abc#def" &&
					os.Getenv("ZUUL_DEFAULT_TENANT") == "my tenant" &&
					os.Getenv("ZUUL_HTTP_HOST") == "a # b"
			},
		},
		{
			name:    "trailing whitespace after unquoted values is dropped",
			content: "ZUUL_HTTP_PORT=8080 \t\nZUUL_AUTH_TOKEN=abc  \r\nZUUL_DEFAULT_TENANT=' keep '  ",
			check: func() bool {
				return os.Getenv("ZUUL_HTTP_PORT") == "8080" &&
					os.Getenv("ZUUL_AUTH_TOKEN") == "abc" &&
					os.Getenv("ZUUL_DEFAULT_TENANT") == " keep "
			},
		},
		{
			name:    "comment-only values are empty, a leading hash is literal",
			content: "ZUUL_LOG_LEVEL= # default\nZUUL_DEFAULT_TENANT=\t#x\nZUUL_AUTH_TOKEN=#literal",
			check: func() bool {
				level, levelSet := os.LookupEnv("ZUUL_LOG_LEVEL")
				tenant, tenantSet := os.LookupEnv("ZUUL_DEFAULT_TENANT")
				return levelSet && level == "" && tenantSet && tenant == "" &&
					os.Getenv("ZUUL_AUTH_TOKEN") == "#literal"
			},
		},
		{
			name:    "BOM before an indented comment",
			content: "\ufeff  # comment\nZUUL_URL=https://zuul.example.com",
			check: func() bool {
				return os.Getenv("ZUUL_URL") == "https://zuul.example.com"
			},
		},
		{
			name:    "CRLF line endings",
			content: "ZUUL_URL=https://zuul.example.com\r\nZUUL_DEFAULT_TENANT='t'\r\n",
			check: func() bool {
				return os.Getenv("ZUUL_URL") == "https://zuul.example.com" && os.Getenv("ZUUL_DEFAULT_TENANT") == "t"
			},
		},
		{
			name:    "line of exactly the maximum length",
			content: "ENV_FILE_TEST_VAR=" + strings.Repeat("x", maxEnvLine-len("ENV_FILE_TEST_VAR=")) + "\r\n",
			check: func() bool {
				return len(os.Getenv("ENV_FILE_TEST_VAR")) == maxEnvLine-len("ENV_FILE_TEST_VAR=")
			},
		},
		{
			name:    "line one byte over the maximum (LF)",
			content: "ENV_FILE_TEST_VAR=" + strings.Repeat("x", maxEnvLine+1-len("ENV_FILE_TEST_VAR=")) + "\n",
			wantErr: true,
		},
		{
			name:    "line one byte over the maximum (CRLF)",
			content: "ENV_FILE_TEST_VAR=" + strings.Repeat("x", maxEnvLine+1-len("ENV_FILE_TEST_VAR=")) + "\r\n",
			wantErr: true,
		},
		{
			name:        "line one byte over the maximum (EOF)",
			content:     "ENV_FILE_TEST_VAR=" + strings.Repeat("x", maxEnvLine+1-len("ENV_FILE_TEST_VAR=")),
			wantErr:     true,
			errContains: "longer than",
		},
		{
			name:    "empty value",
			content: `ZUUL_DEFAULT_TENANT=`,
			check: func() bool {
				v, ok := os.LookupEnv("ZUUL_DEFAULT_TENANT")
				return ok && v == ""
			},
		},
		{
			name:        "unterminated quote does not echo the value",
			content:     `ZUUL_AUTH_TOKEN="s3cr3t`,
			wantErr:     true,
			errContains: "unterminated",
		},
		{
			name:        "text after quoted value",
			content:     `ZUUL_AUTH_TOKEN="s3cr3t" extra`,
			wantErr:     true,
			errContains: "after quoted value",
		},
		{
			name:    "long value",
			content: "ENV_FILE_TEST_VAR=" + strings.Repeat("x", 200<<10),
			check: func() bool {
				return len(os.Getenv("ENV_FILE_TEST_VAR")) == 200<<10
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateEnv(t, tt.presetEnv)

			envFile := filepath.Join(t.TempDir(), ".env")
			if err := os.WriteFile(envFile, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			err := LoadEnvFile(envFile)

			if tt.wantErr {
				switch {
				case err == nil:
					t.Error("LoadEnvFile() expected error, got nil")
				case tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains):
					t.Errorf("LoadEnvFile() error = %v, want error containing %q", err, tt.errContains)
				case strings.Contains(err.Error(), "s3cr3t"):
					t.Errorf("LoadEnvFile() error leaks the value: %v", err)
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
	if err := LoadEnvFile("/nonexistent/path/.env"); err == nil {
		t.Error("LoadEnvFile() expected error for non-existent file")
	}
}

func writeSecret(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_SecretFiles(t *testing.T) {
	t.Run("tokens from files", func(t *testing.T) {
		isolateEnv(t, map[string]string{
			"ZUUL_AUTH_TOKEN_FILE":      writeSecret(t, "zuul-jwt\n"),
			"ZUUL_HTTP_AUTH_TOKEN_FILE": writeSecret(t, "  client-token  \n"),
		})
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AuthToken != "zuul-jwt" || cfg.HTTPAuthToken != "client-token" {
			t.Errorf("tokens = %q, %q", cfg.AuthToken, cfg.HTTPAuthToken)
		}
	})

	t.Run("symlinked secret", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), "token")
		if err := os.Symlink(writeSecret(t, "zuul-jwt"), link); err != nil {
			t.Skipf("symlink: %v", err)
		}
		isolateEnv(t, map[string]string{"ZUUL_AUTH_TOKEN_FILE": link})
		cfg, err := Load()
		if err != nil || cfg.AuthToken != "zuul-jwt" {
			t.Fatalf("Load() = %v, %v", cfg, err)
		}
	})

	tests := []struct {
		name    string
		env     func(t *testing.T) map[string]string
		wantErr string
	}{
		{"both set", func(t *testing.T) map[string]string {
			return map[string]string{"ZUUL_AUTH_TOKEN": "s3cr3t", "ZUUL_AUTH_TOKEN_FILE": writeSecret(t, "s3cr3t")}
		}, "only one of ZUUL_AUTH_TOKEN and ZUUL_AUTH_TOKEN_FILE"},
		{"empty variable and file both set", func(t *testing.T) map[string]string {
			return map[string]string{"ZUUL_HTTP_AUTH_TOKEN": "", "ZUUL_HTTP_AUTH_TOKEN_FILE": writeSecret(t, "s3cr3t")}
		}, "only one of ZUUL_HTTP_AUTH_TOKEN and ZUUL_HTTP_AUTH_TOKEN_FILE"},
		{"directory", func(t *testing.T) map[string]string {
			return map[string]string{"ZUUL_AUTH_TOKEN_FILE": t.TempDir()}
		}, "regular file"},
		{"missing file", func(t *testing.T) map[string]string {
			return map[string]string{"ZUUL_AUTH_TOKEN_FILE": filepath.Join(t.TempDir(), "nope")}
		}, "reading ZUUL_AUTH_TOKEN_FILE"},
		{"empty file", func(t *testing.T) map[string]string {
			return map[string]string{"ZUUL_HTTP_AUTH_TOKEN_FILE": writeSecret(t, " \n")}
		}, "ZUUL_HTTP_AUTH_TOKEN_FILE is empty"},
		{"oversized file", func(t *testing.T) map[string]string {
			return map[string]string{"ZUUL_AUTH_TOKEN_FILE": writeSecret(t, strings.Repeat("s3cr3t", 20<<10))}
		}, "larger than"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateEnv(t, tt.env(t))
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want %q", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), "s3cr3t") {
				t.Errorf("error leaks the secret: %v", err)
			}
		})
	}
}

func TestLoad_LogLevel(t *testing.T) {
	base := map[string]string{"ZUUL_URL": "https://zuul.example.com"}
	isolateEnv(t, base)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Level() != slog.LevelInfo {
		t.Errorf("default log level = %v, want info", cfg.Level())
	}

	isolateEnv(t, map[string]string{"ZUUL_URL": base["ZUUL_URL"], "ZUUL_LOG_LEVEL": "DEBUG"})
	if cfg, err = Load(); err != nil || cfg.Validate() != nil || cfg.Level() != slog.LevelDebug {
		t.Errorf("Load() = %v, %v; want debug", cfg, err)
	}

	// An invalid environment value is reported by Validate, so a valid
	// -log-level flag applied before Validate can override it.
	isolateEnv(t, map[string]string{"ZUUL_URL": base["ZUUL_URL"], "ZUUL_LOG_LEVEL": "loud"})
	if cfg, err = Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "ZUUL_LOG_LEVEL") {
		t.Errorf("Validate() error = %v, want ZUUL_LOG_LEVEL error", err)
	}
	cfg.LogLevel = "warn"
	if err := cfg.Validate(); err != nil || cfg.Level() != slog.LevelWarn {
		t.Errorf("after override: Validate() = %v, Level() = %v; want nil, warn", err, cfg.Level())
	}
}

func TestParseLogLevel(t *testing.T) {
	for in, want := range map[string]slog.Level{
		"debug": slog.LevelDebug, " Info ": slog.LevelInfo, "warn": slog.LevelWarn, //nolint:gocritic // checks trimming and case folding
		"error": slog.LevelError,
	} {
		if got, err := ParseLogLevel(in); err != nil || got != want {
			t.Errorf("ParseLogLevel(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "trace", "warning", "debug+1", "-4"} {
		if _, err := ParseLogLevel(in); err == nil {
			t.Errorf("ParseLogLevel(%q) should fail", in)
		}
	}
}
