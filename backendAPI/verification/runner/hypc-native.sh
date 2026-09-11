#!/bin/sh
# Manual native-hypc compatibility runner.
#
# The backend verifier does not execute this script. Enabled native builds are
# copied into a sealed memfd and executed directly from the retained descriptor.
# This utility preserves the historical runner wire contract for manual checks:
#
#   HYPC_NODE_BIN=/bin/sh                  (was "node")
#   HYPC_RUNNER=.../hypc-native.sh         (was "hypc-runner.js")
#
# Contract:
#   $0 --version  →  stdout = single line with the hypc build id, no prefix
#   $0            →  stdin = standard-JSON, stdout = standard-JSON output
#
# HYPC_BIN must name an absolute path selected by the caller.
# There is deliberately no PATH fallback for native compiler discovery.

set -eu

: "${HYPC_BIN:?HYPC_BIN is required and must be an absolute path}"

case "$HYPC_BIN" in
    /*) ;;
    *)
        echo "hypc-native: HYPC_BIN must be an absolute path" >&2
        exit 64
        ;;
esac

if [ ! -x "$HYPC_BIN" ]; then
    echo "hypc-native: HYPC_BIN is not executable: $HYPC_BIN" >&2
    exit 69
fi

bin="$HYPC_BIN"

if [ "${1-}" = "--version" ]; then
    # `hypc --version` prints two lines; only the second carries the build
    # id. Strip the "Version: " prefix so the output matches the format
    # the WASM runner emits ("0.0.2+commit.3e18e55d.Emscripten.clang").
    "$bin" --version | awk '/^Version:/{sub(/^Version: */, ""); print; exit}'
    exit 0
fi

exec "$bin" --standard-json
