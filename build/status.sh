#!/usr/bin/env bash
# Workspace status script for Bazel stamping.
# Outputs stable keys used by x_defs in go_binary rules.

echo "STABLE_GIT_TAG $(git describe --tags --always --dirty 2>/dev/null || echo 'dev')"
echo "STABLE_GIT_COMMIT $(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
echo "BUILD_DATE $(date -u +%Y-%m-%dT%H:%M:%SZ)"
