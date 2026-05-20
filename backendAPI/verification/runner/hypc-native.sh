#!/bin/sh
# Native-hypc runner.
#
# Matches the wire contract of hypc-runner.js so the Go side
# (backendAPI/verification/compiler.go) can swap between the WASM-via-Node
# runner and a native hypc binary by changing two env vars:
#
#   HYPC_NODE_BIN=/bin/sh                  (was "node")
#   HYPC_RUNNER=.../hypc-native.sh         (was "hypc-runner.js")
#
# Contract:
#   $0 --version  →  stdout = single line with the hypc build id, no prefix
#   $0            →  stdin = standard-JSON, stdout = standard-JSON output
#
# HYPC_BIN env var overrides the default `hypc` discovery (lets us pin the
# absolute path on a deploy host without depending on PATH ordering).

set -eu

bin="${HYPC_BIN:-hypc}"

if [ "${1-}" = "--version" ]; then
    # `hypc --version` prints two lines; only the second carries the build
    # id. Strip the "Version: " prefix so the output matches the format
    # the WASM runner emits ("0.0.2+commit.3e18e55d.Emscripten.clang").
    "$bin" --version | awk '/^Version:/{sub(/^Version: */, ""); print; exit}'
    exit 0
fi

exec "$bin" --standard-json
