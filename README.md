# Zuul MCP Server

[![Release](https://img.shields.io/github/v/release/clappingmonkey/zuul-mcp)](https://github.com/clappingmonkey/zuul-mcp/releases)

A Model Context Protocol (MCP) server that enables AI applications like Claude to interact with [Zuul CI/CD](https://zuul-ci.org/) systems.

## Features

- **16 MCP Tools** for comprehensive Zuul interaction:
  - `list_tenants` - List all Zuul tenants
  - `list_builds` - Query builds with filters (project, pipeline, branch, result, etc.)
  - `get_build` - Get build details by UUID
  - `get_build_logs` - Get job output logs for a build
  - `list_buildsets` - Query buildsets with filters
  - `list_jobs` - List jobs in a tenant
  - `get_job` - Get job details
  - `list_pipelines` - List pipelines
  - `get_pipeline_status` - Get current pipeline status including queue
  - `list_projects` - List projects
  - `get_project` - Get project details
  - `get_tenant_status` - Get overall tenant status
  - `get_config_errors` - Get configuration errors
  - `list_autoholds` - List autohold requests (requires auth)
  - `create_autohold` - Create autohold request (requires auth)
  - `delete_autohold` - Delete autohold request (requires auth)

- **Multiple Transport Modes**: stdio (for Claude Desktop), HTTP, and SSE
- **Cross-Platform**: Native binaries for Linux, macOS, and Windows
- **Optional Authentication**: JWT Bearer tokens for authenticated endpoints

## Installation

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

### Build from Source

```bash
# Requires Go 1.23+
go install github.com/clappingmonkey/zuul-mcp/cmd/zuul-mcp@latest
```

## Configuration

Configuration is done via environment variables:

| Variable | Required | Description |
|----------|----------|-------------|
| `ZUUL_URL` | Yes | Base URL of your Zuul instance (e.g., `https://zuul.example.com`) |
| `ZUUL_DEFAULT_TENANT` | No | Default tenant to use if not specified in tool calls |
| `ZUUL_AUTH_TOKEN` | No | JWT Bearer token for authenticated endpoints (autoholds) |
| `ZUUL_TRANSPORT` | No | Transport mode: `stdio` (default), `http`, or `sse` |
| `ZUUL_HTTP_PORT` | No | HTTP/SSE server port (default: `8080`) |

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

For authenticated operations (autoholds):

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

## Usage with HTTP Transport

For remote or web-based access:

```bash
# Start the server
ZUUL_URL=https://zuul.example.com ZUUL_TRANSPORT=http zuul-mcp

# Or with command line flags
zuul-mcp -transport=http -port=8080
```

## Command Line Options

```bash
zuul-mcp [options]
```

| Flag | Description |
|------|-------------|
| `-version` | Show version information and exit |
| `-transport` | Transport mode: `stdio` (default), `http`, or `sse` |
| `-port` | HTTP/SSE server port (default: `8080`) |

Example version output:

```
zuul-mcp 0.2.0 (abc1234) built on 2024-01-15T10:30:00Z
```

## Example Prompts for Claude

Once configured, you can ask Claude questions like:

- "List all tenants in the Zuul instance"
- "Show me the recent failed builds for project openstack/nova"
- "What is the current status of the gate pipeline?"
- "Are there any configuration errors in the openstack tenant?"
- "Show me details of build UUID abc123..."
- "Create an autohold for job my-failing-job in project my-project"

## Development

This project uses Bazel for building, testing, and dependency management. No `go.mod` file is needed - all dependencies are declared in `MODULE.bazel`.

### Prerequisites

- [Bazel](https://bazel.build/install) or [Bazelisk](https://github.com/bazelbuild/bazelisk) (recommended)
- Go 1.23+ (for IDE support/gopls only - Bazel manages its own Go SDK)

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

1. Add a `go_deps.module()` declaration to `MODULE.bazel`:
   ```starlark
   go_deps.module(
       path = "github.com/example/package",
       sum = "h1:...",  # Get from go.sum after `go get`
       version = "v1.0.0",
   )
   ```

2. Add the repository name to the `use_repo()` call

3. Run `bazel run //:gazelle` to update BUILD files

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.
