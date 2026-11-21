# Zuul MCP Server

[![Release](https://img.shields.io/github/v/release/clappingmonkey/zuul-mcp)](https://github.com/clappingmonkey/zuul-mcp/releases)
[![clappingmonkey/zuul-mcp MCP server](https://glama.ai/mcp/servers/clappingmonkey/zuul-mcp/badges/score.svg)](https://glama.ai/mcp/servers/clappingmonkey/zuul-mcp)

A Model Context Protocol (MCP) server that enables AI applications like Claude to interact with [Zuul CI/CD](https://zuul-ci.org/) systems.

## Features

- **28 MCP Tools** for comprehensive Zuul interaction:
  - `list_tenants` - List all Zuul tenants
  - `list_builds` - Query builds with filters (project, pipeline, branch, result, etc.)
  - `get_build` - Get build details by UUID
  - `get_build_logs` - Get the end of a build's job output log (size-capped)
  - `list_buildsets` - Query buildsets with filters
  - `get_buildset` - Get buildset details by UUID
  - `list_jobs` - List jobs in a tenant
  - `get_job` - Get job details (all variants of the job)
  - `list_pipelines` - List pipelines
  - `get_pipeline_status` - Get current pipeline status including queue
  - `list_projects` - List projects
  - `get_project` - Get project details
  - `get_tenant_status` - Get overall tenant status
  - `get_change_status` - Get pipeline status for a specific change
  - `get_config_errors` - Get configuration errors and warnings (filterable, paged)
  - `list_system_events` - List system events (config updates, reconfigurations)
  - `list_nodes` - List nodepool nodes
  - `list_labels` - List available node labels
  - `list_connections` - List all Zuul connections
  - `list_components` - List all Zuul components (schedulers, executors, mergers, etc.)
  - `list_semaphores` - List semaphores
  - `list_autoholds` - List autohold requests
  - `get_autohold` - Get details of a specific autohold request
  - `create_autohold` - Create autohold request (requires auth)
  - `delete_autohold` - Delete autohold request (requires auth)
  - `enqueue` - Enqueue a change or ref into a pipeline (requires auth)
  - `dequeue` - Dequeue a change or ref from a pipeline (requires auth)
  - `promote` - Promote changes to the top of a pipeline queue (requires auth)

- **Safe by default**: the five write tools are only offered when `ZUUL_AUTH_TOKEN` is set and read-only mode is off. Every tool carries MCP annotations (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`) so clients can ask for confirmation before state-changing calls.
- **Multiple Transport Modes**: stdio (for Claude Desktop), HTTP, and SSE
- **Cross-Platform**: Native binaries for Linux, macOS, and Windows
- **Optional Authentication**: JWT Bearer tokens for authenticated endpoints

## Installation

### Homebrew (macOS and Linux)

```bash
brew tap clappingmonkey/zuul-mcp
brew install zuul-mcp
```

### Download Binary

Download the latest release for your platform from the [releases page](https://github.com/clappingmonkey/zuul-mcp/releases).

```bash
# Linux (amd64)
curl -LO https://github.com/clappingmonkey/zuul-mcp/releases/latest/download/zuul-mcp-linux-amd64
chmod +x zuul-mcp-linux-amd64
sudo mv zuul-mcp-linux-amd64 /usr/local/bin/zuul-mcp

# macOS (Apple Silicon)
curl -LO https://github.com/clappingmonkey/zuul-mcp/releases/latest/download/zuul-mcp-darwin-arm64
chmod +x zuul-mcp-darwin-arm64
sudo mv zuul-mcp-darwin-arm64 /usr/local/bin/zuul-mcp

# macOS (Intel)
curl -LO https://github.com/clappingmonkey/zuul-mcp/releases/latest/download/zuul-mcp-darwin-amd64
chmod +x zuul-mcp-darwin-amd64
sudo mv zuul-mcp-darwin-amd64 /usr/local/bin/zuul-mcp
```

Windows: download `zuul-mcp-windows-amd64.exe` from the same page.

Each release includes `checksums.txt` (SHA-256) and GitHub build provenance
attestations. To check a downloaded binary was built by this repository's
release workflow:

```bash
gh attestation verify zuul-mcp-linux-amd64 --repo clappingmonkey/zuul-mcp
```

### Build from Source

```bash
# Requires the Go version in go.mod (1.27.4) or newer; with the default
# GOTOOLCHAIN=auto an older go downloads it automatically.
go install github.com/clappingmonkey/zuul-mcp/cmd/zuul-mcp@latest
```

## Configuration

Configuration can be done via environment variables or an env file:

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ZUUL_URL` | Yes | Base URL of your Zuul instance (e.g., `https://zuul.example.com`) |
| `ZUUL_DEFAULT_TENANT` | No | Default tenant to use if not specified in tool calls |
| `ZUUL_AUTH_TOKEN` | No | JWT Bearer token for write tools (create/delete autohold, enqueue, dequeue, promote). Without it the write tools are not offered |
| `ZUUL_READ_ONLY` | No | Set to `true` to hide the write tools even when `ZUUL_AUTH_TOKEN` is set (same as `-read-only`) |
| `ZUUL_TRANSPORT` | No | Transport mode: `stdio` (default), `http`, or `sse` |
| `ZUUL_HTTP_PORT` | No | HTTP/SSE server port (default: `8080`) |
| `ZUUL_HTTP_HOST` | No | HTTP/SSE bind address (default: `127.0.0.1`) |
| `ZUUL_HTTP_AUTH_TOKEN` | No | Bearer token (min. 32 characters) clients must send to the HTTP/SSE transport. Required when `ZUUL_HTTP_HOST` is not a loopback address |
| `ZUUL_HTTP_ALLOWED_ORIGINS` | No | Comma-separated browser origins (`scheme://host[:port]`) allowed to call the HTTP/SSE transport. Requests with any other `Origin` header are rejected |
| `ZUUL_ALLOW_INSECURE_HTTP` | No | Set to `true` to allow sending `ZUUL_AUTH_TOKEN` to a non-loopback `http://` `ZUUL_URL` (not recommended) |
| `ZUUL_LOG_LEVEL` | No | `debug`, `info` (default), `warn` or `error`. Logs always go to stderr |

`ZUUL_AUTH_TOKEN` and `ZUUL_HTTP_AUTH_TOKEN` can instead be read from a file by
setting `ZUUL_AUTH_TOKEN_FILE` / `ZUUL_HTTP_AUTH_TOKEN_FILE` to its path (for
example a Docker or Kubernetes secret). Surrounding whitespace is trimmed; set
either the variable or the `_FILE` variant, not both.

### Using an Env File

You can use the `--env-file` flag to load configuration from a file instead of environment variables:

```bash
zuul-mcp --env-file /path/to/.env
```

The env file format is simple key-value pairs:

```env
# .env.zuul
ZUUL_URL=https://zuul.example.com  # comments may follow unquoted values
export ZUUL_DEFAULT_TENANT=openstack
ZUUL_AUTH_TOKEN='your-jwt-token'
```

Lines may start with `export `. Single- or double-quoted values are taken
literally (no escape sequences or variable expansion); an unquoted value ends
at ` #`.

**Note**: Variables already set in the environment, even to an empty value, take precedence over the env file. This allows you to override specific settings without modifying the file.

## Usage with Claude Desktop

Add to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "zuul": {
      "command": "/usr/local/bin/zuul-mcp",
      "env": {
        "ZUUL_URL": "https://zuul.example.com",
        "ZUUL_DEFAULT_TENANT": "openstack"
      }
    }
  }
}
```

For authenticated operations (autoholds, queue management):

```json
{
  "mcpServers": {
    "zuul": {
      "command": "/usr/local/bin/zuul-mcp",
      "env": {
        "ZUUL_URL": "https://zuul.example.com",
        "ZUUL_DEFAULT_TENANT": "openstack",
        "ZUUL_AUTH_TOKEN": "your-jwt-token"
      }
    }
  }
}
```

Alternatively, using an env file (useful for keeping secrets out of the config):

```json
{
  "mcpServers": {
    "zuul": {
      "command": "/usr/local/bin/zuul-mcp",
      "args": ["--env-file", "/path/to/.env.zuul"]
    }
  }
}
```

## Usage with HTTP Transport

For remote or web-based access:

```bash
# Start the server on http://127.0.0.1:8080/mcp
ZUUL_URL=https://zuul.example.com ZUUL_TRANSPORT=http zuul-mcp

# Or with command line flags
zuul-mcp -transport=http -port=8080
```

The HTTP and SSE transports listen on `127.0.0.1` by default. Requests without
an `Origin` header (CLI and desktop MCP clients) are accepted. Browser requests
are rejected unless their exact origin is listed in `ZUUL_HTTP_ALLOWED_ORIGINS`.
That includes local pages: to use the MCP Inspector, for example, set
`ZUUL_HTTP_ALLOWED_ORIGINS=http://localhost:6274`.

To listen on another interface you must also set `ZUUL_HTTP_AUTH_TOKEN` (at
least 32 characters). Clients then have to send `Authorization: Bearer <token>`.
Anyone who can reach the server acts with your `ZUUL_AUTH_TOKEN`, so put TLS (a
reverse proxy) in front of it:

```bash
ZUUL_HTTP_AUTH_TOKEN="$(openssl rand -hex 32)" zuul-mcp -transport=http -host=0.0.0.0
```

Without a token, connections on `127.0.0.1` must also carry a loopback `Host`
header (DNS rebinding protection). A reverse proxy on the same machine that
forwards its public `Host` therefore needs `ZUUL_HTTP_AUTH_TOKEN` as well.

Browsers cannot send an `Authorization` header with the legacy SSE transport's
`EventSource`, so use `-transport=http` for browser clients that need a token.

## Command Line Options

```bash
zuul-mcp [options]
```

| Flag | Description |
|------|-------------|
| `-env-file` | Path to `.env` file for configuration |
| `-transport` | Transport mode: `stdio` (default), `http`, or `sse` |
| `-host` | HTTP/SSE bind address (default: `127.0.0.1`) |
| `-port` | HTTP/SSE server port (default: `8080`) |
| `-read-only` | Hide tools that change Zuul state |
| `-log-level` | `debug`, `info` (default), `warn` or `error` |
| `-version` | Show version information and exit |

The server stops cleanly on `SIGINT`/`SIGTERM` (exit status 0). The HTTP/SSE
transports stop accepting connections, give in-flight requests and event
streams 2 seconds to finish, then cancel them; connections still open after
10 seconds are closed. The stdio transport waits up to 5 seconds for pending
responses. A second signal stops the server immediately.

Example version output:

```
zuul-mcp 0.14.1 (a1d9e05) built on 2026-11-19T10:30:00Z
```

## Example Prompts for Claude

Once configured, you can ask Claude questions like:

- "List all tenants in the Zuul instance"
- "Show me the recent failed builds for project openstack/nova"
- "What is the current status of the gate pipeline?"
- "What's the pipeline status for change 12345,1?"
- "Are there any configuration errors in the openstack tenant?"
- "Show me recent system events in this tenant"
- "Show me details of build UUID abc123..."
- "What Zuul components are running and are they healthy?"
- "Create an autohold for job my-failing-job in project my-project"
- "Show me details of autohold request 42"
- "What node labels are available in this tenant?"
- "Show me all variants of job build-container"
- "List all connections configured in Zuul"
- "What semaphores are defined in the tenant?"
- "Enqueue change 12345,1 into the gate pipeline for project my-project"
- "Promote changes 12345,1 and 13336,3 in the gate pipeline"

## Development

This project builds and tests with Bazel only (do not use `go build`/`go test`). Go dependencies are declared in `go.mod`/`go.sum`, which Bazel reads via `go_deps.from_file()`; Bazel module dependencies are in `MODULE.bazel`.

### Prerequisites

- [Bazel](https://bazel.build/install) or [Bazelisk](https://github.com/bazelbuild/bazelisk) (recommended)
- Go matching the `go` line in `go.mod` (only for `go get`/`go mod tidy` and IDE/gopls support - Bazel manages its own Go SDK)

### Build

```bash
# Build the binary (development)
bazel build //cmd/zuul-mcp

# Build with version stamping (for releases)
bazel build //cmd/zuul-mcp --config=release

# The binary will be at:
# bazel-bin/cmd/zuul-mcp/zuul-mcp_/zuul-mcp
```

### Test

```bash
# Run all tests
bazel test //...
```

### Lint and Git Hooks

```bash
# golangci-lint (pinned, runs with Bazel's Go SDK)
bazel run //tools/lint:golangci_lint -- run ./...

# Install the git hooks (prek or pre-commit), or run them on demand
prek install
prek run --all-files
```

See [CONTRIBUTING.md](CONTRIBUTING.md#git-hooks-and-linting) for details.

### Regenerate BUILD files

```bash
# After adding new Go files or packages
bazel run //:gazelle
```

### Cross-Compile

```bash
# Linux amd64 (statically linked)
bazel build //cmd/zuul-mcp --config=linux_amd64

# Linux arm64 (statically linked)
bazel build //cmd/zuul-mcp --config=linux_arm64

# macOS amd64
bazel build //cmd/zuul-mcp --config=darwin_amd64

# macOS arm64 (Apple Silicon)
bazel build //cmd/zuul-mcp --config=darwin_arm64

# Windows amd64
bazel build //cmd/zuul-mcp --config=windows_amd64
```

### Adding Dependencies

To add a new Go dependency:

1. Add the dependency using Go tooling:
   ```bash
   go get github.com/example/package@v1.0.0
   go mod tidy
   ```

2. Update Bazel module configuration:
   ```bash
   bazel mod tidy         # auto-fix use_repo() if needed
   bazel run //:gazelle   # regenerate BUILD files
   ```

3. Verify everything works:
   ```bash
   bazel test //...
   ```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## Security

To report a vulnerability, see [SECURITY.md](SECURITY.md).
