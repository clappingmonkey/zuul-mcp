// Package config provides configuration management for the Zuul MCP server.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
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

	// HTTPHost is the interface the HTTP/SSE transport binds to. Defaults to
	// loopback so the server (and the Zuul token it holds) is not exposed to
	// the network by accident.
	HTTPHost string

	// HTTPAuthToken, when set, is the bearer token MCP clients must present to
	// the HTTP/SSE transport. Required when binding to a non-loopback address.
	HTTPAuthToken string

	// HTTPAllowedOrigins lists the canonical browser origins
	// (scheme://host[:port]) allowed to call the HTTP/SSE transport. Requests
	// carrying any other Origin header are rejected.
	HTTPAllowedOrigins []string

	// AllowInsecureHTTP permits sending AuthToken to a non-loopback Zuul over
	// plain http://.
	AllowInsecureHTTP bool
}

// Load reads configuration from environment variables and applies defaults.
// Callers must call Validate after applying any overrides.
func Load() (*Config, error) {
	cfg := &Config{
		ZuulURL:       os.Getenv("ZUUL_URL"),
		DefaultTenant: os.Getenv("ZUUL_DEFAULT_TENANT"),
		AuthToken:     os.Getenv("ZUUL_AUTH_TOKEN"),
		Transport:     os.Getenv("ZUUL_TRANSPORT"),
		HTTPPort:      os.Getenv("ZUUL_HTTP_PORT"),
		HTTPHost:      os.Getenv("ZUUL_HTTP_HOST"),
		HTTPAuthToken: os.Getenv("ZUUL_HTTP_AUTH_TOKEN"),
	}
	for i, origin := range strings.Split(os.Getenv("ZUUL_HTTP_ALLOWED_ORIGINS"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			canonical, err := CanonicalOrigin(origin)
			if err != nil {
				return nil, fmt.Errorf("ZUUL_HTTP_ALLOWED_ORIGINS entry %d: %w", i+1, err)
			}
			cfg.HTTPAllowedOrigins = append(cfg.HTTPAllowedOrigins, canonical)
		}
	}
	if v := os.Getenv("ZUUL_ALLOW_INSECURE_HTTP"); v != "" {
		allow, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("ZUUL_ALLOW_INSECURE_HTTP must be a boolean: %w", err)
		}
		cfg.AllowInsecureHTTP = allow
	}

	if cfg.Transport == "" {
		cfg.Transport = "stdio"
	}
	if cfg.HTTPPort == "" {
		cfg.HTTPPort = "8080"
	}
	if cfg.HTTPHost == "" {
		cfg.HTTPHost = "127.0.0.1"
	}
	return cfg, nil
}

// Validate checks the effective configuration, including settings that would
// otherwise leak credentials or expose the server. Call it after applying
// command-line overrides.
func (c *Config) Validate() error {
	if c.ZuulURL == "" {
		return errors.New("ZUUL_URL environment variable is required")
	}
	u, err := url.Parse(c.ZuulURL)
	if err != nil {
		// url.Parse errors quote the input, which may contain credentials.
		return errors.New("ZUUL_URL is not a valid URL")
	}
	if u.User != nil {
		return errors.New("ZUUL_URL must not contain credentials; use ZUUL_AUTH_TOKEN")
	}
	// Error messages below never echo the URL: its query, fragment or path
	// could carry secrets.
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return errors.New("ZUUL_URL must be an absolute http:// or https:// URL")
	}
	if p := u.Port(); p != "" && !validPort(p) {
		return errors.New("ZUUL_URL has an invalid port")
	}
	if u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
		return errors.New("ZUUL_URL must not contain a query or fragment")
	}
	if c.HasAuth() && u.Scheme == "http" && !IsLoopback(u.Hostname()) && !c.AllowInsecureHTTP {
		return errors.New("refusing to send ZUUL_AUTH_TOKEN over plain http:// to a non-local Zuul; use https:// or set ZUUL_ALLOW_INSECURE_HTTP=true")
	}

	if !validPort(c.HTTPPort) {
		return fmt.Errorf("HTTP port must be a number between 1 and 65535, got %q", c.HTTPPort)
	}
	switch c.Transport {
	case "stdio":
	case "http", "sse":
		if c.HTTPAuthToken != "" && (len(c.HTTPAuthToken) < MinHTTPAuthTokenLength || strings.TrimSpace(c.HTTPAuthToken) != c.HTTPAuthToken) {
			return fmt.Errorf("ZUUL_HTTP_AUTH_TOKEN must be at least %d characters without surrounding whitespace (e.g. openssl rand -hex 32)", MinHTTPAuthTokenLength)
		}
		if !IsLoopback(c.HTTPHost) && c.HTTPAuthToken == "" {
			return fmt.Errorf("refusing to serve %s on non-loopback address %q without ZUUL_HTTP_AUTH_TOKEN: anyone who can reach it could use this server's Zuul credentials", c.Transport, c.HTTPHost)
		}
	default:
		return fmt.Errorf("unknown transport %q (use 'stdio', 'http', or 'sse')", c.Transport)
	}
	return nil
}

// MinHTTPAuthTokenLength is the shortest accepted ZUUL_HTTP_AUTH_TOKEN. The
// transport has no rate limiting, so the token must resist guessing.
const MinHTTPAuthTokenLength = 32

func validPort(s string) bool {
	port, err := strconv.Atoi(s)
	return err == nil && port >= 1 && port <= 65535
}

// IsLoopback reports whether host (without port) is "localhost" or a
// loopback IP address. Anything it does not recognise counts as non-loopback,
// so callers fail closed.
func IsLoopback(host string) bool {
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.Unmap().IsLoopback()
}

// CanonicalOrigin converts a configured origin such as "HTTPS://App.Example/"
// into the serialized form browsers send in the Origin header
// ("https://app.example"), so it can be compared exactly. Errors never echo
// the input, which could contain credentials.
func CanonicalOrigin(origin string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(origin))
	if err != nil {
		return "", errors.New("invalid origin: use scheme://host[:port]")
	}
	if u.User != nil {
		return "", errors.New("invalid origin: must not contain credentials")
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "http" && scheme != "https") || u.Hostname() == "" ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("invalid origin: use scheme://host[:port]")
	}
	host := strings.ToLower(u.Hostname())
	for _, r := range host {
		if r > 0x7f {
			return "", errors.New("invalid origin: write internationalized host names in their punycode (xn--) form")
		}
	}
	if strings.Contains(host, ":") {
		// Browsers serialize IPv6 hosts in their shortest form.
		ip, err := netip.ParseAddr(host)
		if err != nil || ip.Zone() != "" {
			return "", errors.New("invalid origin: bad IPv6 address")
		}
		if ip.Is4In6() {
			// netip renders these with a dotted IPv4 tail; browsers use hex.
			return "", errors.New("invalid origin: IPv4-mapped IPv6 addresses are not supported; use the IPv4 address")
		}
		host = "[" + ip.String() + "]"
	} else if endsInNumber(host) {
		// Browsers rewrite numeric hosts (127.000.000.001, 0x7f.1, ...) to
		// dotted decimal; only accept that form so the comparison matches.
		ip, err := netip.ParseAddr(host)
		if err != nil || !ip.Is4() || ip.String() != host {
			return "", errors.New("invalid origin: write IPv4 addresses in dotted decimal form, e.g. 127.0.0.1")
		}
	}
	port := u.Port()
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", errors.New("invalid origin: bad port")
		}
		port = strconv.Itoa(n)
	}
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		host += ":" + port
	}
	return scheme + "://" + host, nil
}

// endsInNumber reports whether the last dot-separated label of host looks
// numeric (decimal or 0x-hex), which makes browsers parse it as IPv4.
func endsInNumber(host string) bool {
	labels := strings.Split(strings.TrimSuffix(host, "."), ".")
	last := labels[len(labels)-1]
	if last == "" {
		return false
	}
	if strings.HasPrefix(last, "0x") {
		return true
	}
	for _, r := range last {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
