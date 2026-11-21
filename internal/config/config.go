// Package config provides configuration management for the Zuul MCP server.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

	// ReadOnly hides the tools that change Zuul state even when AuthToken
	// is set.
	ReadOnly bool

	// LogLevel is the minimum level written to stderr (ZUUL_LOG_LEVEL):
	// debug, info, warn or error. Empty means info. See Level.
	LogLevel string
}

// Level returns the parsed LogLevel, or info if it is empty or invalid
// (Validate reports invalid values).
func (c *Config) Level() slog.Level {
	if c.LogLevel == "" {
		return slog.LevelInfo
	}
	level, err := ParseLogLevel(c.LogLevel)
	if err != nil {
		return slog.LevelInfo
	}
	return level
}

// maxSecretFileSize bounds the files read for *_FILE secrets.
const maxSecretFileSize = 64 << 10

// envSecret returns the value of key, or the trimmed contents of the file
// named by key+"_FILE" (e.g. a Docker or Kubernetes secret). Setting both is
// an error. Errors never include the secret.
func envSecret(key string) (string, error) {
	value, valueSet := os.LookupEnv(key)
	path := os.Getenv(key + "_FILE")
	if path == "" {
		return value, nil
	}
	if valueSet {
		return "", fmt.Errorf("set only one of %s and %s_FILE", key, key)
	}
	// Only regular files (symlinks to them are fine, as used by Kubernetes
	// secret volumes): reading a FIFO or device could block or never end.
	// The path is checked first, then opened without blocking where the OS
	// supports it, and the opened file is checked again in case it changed.
	notRegular := fmt.Errorf("%s_FILE must point to a regular file", key)
	// The path comes from the operator's own environment, so reading it is
	// the point rather than a traversal risk.
	if info, err := os.Stat(path); err != nil { //nolint:gosec // operator-supplied path
		return "", fmt.Errorf("reading %s_FILE: %w", key, err)
	} else if !info.Mode().IsRegular() {
		return "", notRegular
	}
	f, err := os.OpenFile(path, secretOpenFlags, 0) //nolint:gosec // operator-supplied path
	if err != nil {
		return "", fmt.Errorf("reading %s_FILE: %w", key, err)
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil {
		return "", fmt.Errorf("reading %s_FILE: %w", key, err)
	} else if !info.Mode().IsRegular() {
		return "", notRegular
	}
	data, err := io.ReadAll(io.LimitReader(f, maxSecretFileSize+1))
	if err != nil {
		return "", fmt.Errorf("reading %s_FILE: %w", key, err)
	}
	if len(data) > maxSecretFileSize {
		return "", fmt.Errorf("%s_FILE is larger than %d bytes", key, maxSecretFileSize)
	}
	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", fmt.Errorf("%s_FILE is empty", key)
	}
	return secret, nil
}

// ParseLogLevel parses debug, info, warn or error (case-insensitive).
func ParseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, errors.New("log level must be one of debug, info, warn, error")
}

// envBool parses an optional boolean environment variable (unset or empty
// means false).
func envBool(key string) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return b, nil
}

// Load reads configuration from environment variables and applies defaults.
// Callers must call Validate after applying any overrides.
func Load() (*Config, error) {
	cfg := &Config{
		ZuulURL:       os.Getenv("ZUUL_URL"),
		DefaultTenant: os.Getenv("ZUUL_DEFAULT_TENANT"),
		Transport:     os.Getenv("ZUUL_TRANSPORT"),
		HTTPPort:      os.Getenv("ZUUL_HTTP_PORT"),
		HTTPHost:      os.Getenv("ZUUL_HTTP_HOST"),
		LogLevel:      os.Getenv("ZUUL_LOG_LEVEL"),
	}
	var err error
	if cfg.AuthToken, err = envSecret("ZUUL_AUTH_TOKEN"); err != nil {
		return nil, err
	}
	if cfg.HTTPAuthToken, err = envSecret("ZUUL_HTTP_AUTH_TOKEN"); err != nil {
		return nil, err
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
	if cfg.AllowInsecureHTTP, err = envBool("ZUUL_ALLOW_INSECURE_HTTP"); err != nil {
		return nil, err
	}
	if cfg.ReadOnly, err = envBool("ZUUL_READ_ONLY"); err != nil {
		return nil, err
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
	if c.LogLevel != "" {
		if _, err := ParseLogLevel(c.LogLevel); err != nil {
			return fmt.Errorf("ZUUL_LOG_LEVEL / -log-level: %w", err)
		}
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
//
// Each line is KEY=value, optionally prefixed with "export ". Blank lines and
// lines starting with # are ignored. Values may be wrapped in single or double
// quotes (taken literally, without escape sequences); an unquoted value ends
// at a " #" comment. Variables already present in the environment, even if
// empty, take precedence over the file.
func LoadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Room for a maxEnvLine-byte line plus its CRLF terminator.
	scanner.Buffer(make([]byte, 0, 64<<10), maxEnvLine+2)
	for lineNum := 1; scanner.Scan(); lineNum++ {
		line := scanner.Text()
		if len(line) > maxEnvLine {
			return fmt.Errorf("invalid format at line %d: longer than %d bytes", lineNum, maxEnvLine)
		}
		if lineNum == 1 {
			line = strings.TrimPrefix(line, "\ufeff")
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "export"); ok && rest != "" && (rest[0] == ' ' || rest[0] == '\t') {
			line = strings.TrimSpace(rest)
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid format at line %d: missing '='", lineNum)
		}
		key = strings.TrimSpace(key)
		if !validEnvKey(key) {
			return fmt.Errorf("invalid format at line %d: variable names may only contain letters, digits and '_' and must not start with a digit", lineNum)
		}
		value, err := parseEnvValue(value)
		if err != nil {
			return fmt.Errorf("invalid format at line %d: %w", lineNum, err)
		}

		if _, set := os.LookupEnv(key); set {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading env file: %w", err)
	}
	return nil
}

func validEnvKey(key string) bool {
	if key == "" || (key[0] >= '0' && key[0] <= '9') {
		return false
	}
	for _, r := range key {
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// maxEnvLine is the longest line LoadEnvFile accepts.
const maxEnvLine = 1 << 20

// parseEnvValue unquotes the text after '=' or strips a trailing comment
// (a '#' preceded by whitespace, so "KEY= # note" is empty but "KEY=#x" is
// "#x"). Errors never include the value, which may be a secret.
func parseEnvValue(raw string) (string, error) {
	v := strings.TrimLeft(raw, " \t")
	if v == "" || (v[0] == '#' && len(v) < len(raw)) {
		return "", nil
	}
	if q := v[0]; q == '"' || q == '\'' {
		end := strings.IndexByte(v[1:], q)
		if end < 0 {
			return "", errors.New("unterminated quoted value")
		}
		if rest := strings.TrimSpace(v[end+2:]); rest != "" && !strings.HasPrefix(rest, "#") {
			return "", errors.New("unexpected text after quoted value")
		}
		return v[1 : end+1], nil
	}
	for i := 1; i < len(v); i++ {
		if v[i] == '#' && (v[i-1] == ' ' || v[i-1] == '\t') {
			return strings.TrimSpace(v[:i]), nil
		}
	}
	return strings.TrimRight(v, " \t"), nil
}
