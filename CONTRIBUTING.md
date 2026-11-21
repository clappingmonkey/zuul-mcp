# Contributing to zuul-mcp

Contributions are welcome! This document explains how to get started.

## Development Setup

### Prerequisites

- [Bazel](https://bazel.build/install) or [Bazelisk](https://github.com/bazelbuild/bazelisk) (recommended)
- Go matching the `go` line in `go.mod` (only for `go get`/`go mod tidy` and IDE/gopls support — Bazel manages its own Go SDK)
- Optional: [prek](https://github.com/j178/prek) or [pre-commit](https://pre-commit.com) for the git hooks below

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

## Git Hooks and Linting

The hooks live in [`.pre-commit-config.yaml`](.pre-commit-config.yaml) and work
with either [prek](https://github.com/j178/prek) (a fast, single-binary
drop-in) or [pre-commit](https://pre-commit.com):

```bash
prek install            # or: pre-commit install  (installs pre-commit and commit-msg hooks)
prek run --all-files    # or: pre-commit run --all-files
```

They run file hygiene checks, `gofmt`, Gazelle, golangci-lint, gitleaks,
actionlint, zizmor (GitHub Actions security) and a Conventional Commits check
on the commit message. The Go hooks go through Bazel, so you don't need Go or
golangci-lint installed. To skip a hook once locally, use
`SKIP=<hook-id> git commit ...`.

CI runs the same hooks on every pull request, plus a scan of the whole tree
for secrets. The **Pre-commit hooks** check is required, so a skipped or
failing hook blocks the merge. The full set of required checks on `main` is:

| Check | Workflow |
|-------|----------|
| PR title (Conventional Commits) | `pr-checks.yaml` |
| Bazel / Test | `pr-checks.yaml` → `reusable-bazel.yaml` |
| Bazel / Build | `pr-checks.yaml` → `reusable-bazel.yaml` |
| BUILD files (Gazelle) | `pr-checks.yaml` |
| Pre-commit hooks | `pr-checks.yaml` |

`Vulnerability Scan` (govulncheck) also runs on pull requests, weekly, and on
`main`, but it is informational: a newly published advisory should not block
unrelated PRs.

golangci-lint can also be run directly (configuration in
[`.golangci.yml`](.golangci.yml)):

```bash
bazel run //tools/lint:golangci_lint -- run ./...
```

The golangci-lint release is pinned by version and sha256 in `MODULE.bazel`;
Renovate does not update it, so bump the version and all four hashes together.

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
4. Ensure `bazel test //...` and the hooks (`prek run --all-files`) pass
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
