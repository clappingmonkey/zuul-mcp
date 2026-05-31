# Contributing to zuul-mcp

Contributions are welcome! This document explains how to get started.

## Development Setup

### Prerequisites

- [Bazel](https://bazel.build/install) or [Bazelisk](https://github.com/bazelbuild/bazelisk) (recommended)
- Go 1.26+ (for `go get`/`go mod tidy` and IDE/gopls support — Bazel manages its own Go SDK)

### Build & Test

```bash
bazel build //cmd/zuul-mcp          # build binary
bazel test //...                     # run all tests
bazel run //:gazelle                 # regenerate BUILD files
```

### Important: Bazel-Only Project

- **Do not** use `go build` or `go test` — use Bazel commands instead
- Go dependencies are managed via `go.mod`/`go.sum` (consumed by Bazel via `go_deps.from_file()`)
- Bazel module dependencies (`bazel_dep`) are declared in `MODULE.bazel`

## Adding a Dependency

1. `go get <dep>@<version>`
2. `go mod tidy`
3. `bazel mod tidy` (auto-fixes `use_repo()` if a new dep is added)
4. `bazel run //:gazelle`
5. `bazel test //...`

## Making Changes

1. Fork the repository
2. Create a feature branch off `main`
3. Make your changes (one logical change per PR)
4. Ensure `bazel test //...` passes
5. Run `bazel run //:gazelle` if you added/changed Go files
6. Commit with a [Conventional Commits](https://www.conventionalcommits.org/) message
7. Open a pull request using the [PR template](.github/PULL_REQUEST_TEMPLATE.md)

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add list_images MCP tool
fix: handle nil response in GetBuild
docs: update README with new tools
chore: update dependency versions
```

## Code Style

- Standard Go formatting (`gofmt`)
- Follow existing patterns in the codebase
- Add unit tests for new client methods (see `internal/client/client_test.go`)
- Use `json.RawMessage` for undocumented Zuul API responses

## AI-Assisted Code

If you use AI tools to generate code, please disclose this in the PR checklist. This is for transparency, not a barrier to contribution.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
