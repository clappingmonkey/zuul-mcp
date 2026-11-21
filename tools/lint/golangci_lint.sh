#!/usr/bin/env bash
# Runs the pinned golangci-lint from the workspace root with Bazel's Go SDK on
# PATH, so contributors do not need a local Go or golangci-lint install.

# --- begin runfiles.bash initialization v3 ---
# Copy-pasted from the Bazel Bash runfiles library v3.
set -uo pipefail; set +e; f=bazel_tools/tools/bash/runfiles/runfiles.bash
# shellcheck disable=SC1090
source "${RUNFILES_DIR:-/dev/null}/$f" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "${RUNFILES_MANIFEST_FILE:-/dev/null}" | cut -f2- -d' ')" 2>/dev/null || \
  source "$0.runfiles/$f" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "$0.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "$0.exe.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null || \
  { echo>&2 "ERROR: cannot find $f"; exit 1; }; f=; set -e
# --- end runfiles.bash initialization v3 ---

if [[ -z "${BUILD_WORKSPACE_DIRECTORY:-}" ]]; then
  echo "ERROR: run this with 'bazel run //tools/lint:golangci_lint -- <args>'" >&2
  exit 1
fi

go_bin="$(rlocation "${GO_RLOCATION}")"
lint_bin="$(rlocation "${GOLANGCI_LINT_RLOCATION}")"
if [[ ! -x "${go_bin}" || ! -x "${lint_bin}" ]]; then
  echo "ERROR: could not locate go (${go_bin}) or golangci-lint (${lint_bin}) in runfiles" >&2
  exit 1
fi

# The go wrapper from rules_go resolves the real SDK through runfiles, so it
# needs these exported once golangci-lint invokes it as a child process.
runfiles_export_envvars
export PATH="$(dirname "${go_bin}"):${PATH}"
# Without this the rules_go wrapper runs every go command in the directory
# 'bazel run' was started from, instead of the one golangci-lint asks for.
unset BUILD_WORKING_DIRECTORY

cd "${BUILD_WORKSPACE_DIRECTORY}"
exec "${lint_bin}" "$@"
