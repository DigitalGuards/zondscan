#!/usr/bin/env bash
set -euo pipefail

# Builds the QRL Connect dApp example and stages it into public/dapp-example/
# so Next.js serves it at /dapp-example. Runs as a prebuild step.
#
# Override the source with QRL_CONNECT_REPO / QRL_CONNECT_REF env vars if needed.
# Set SKIP_DAPP_EXAMPLE=1 to bypass entirely (useful for fast local builds).

if [[ "${SKIP_DAPP_EXAMPLE:-0}" == "1" ]]; then
  echo "[dapp-example] SKIP_DAPP_EXAMPLE=1 — skipping build"
  exit 0
fi

REPO_URL="${QRL_CONNECT_REPO:-https://github.com/DigitalGuards/myqrlwallet-connect.git}"
REF="${QRL_CONNECT_REF:-dev}"
LOCAL_SRC="${QRL_CONNECT_LOCAL:-}"

FRONTEND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CACHE_DIR="$FRONTEND_DIR/.dapp-example-cache"
OUT_DIR="$FRONTEND_DIR/public/dapp-example"

echo "[dapp-example] cache=$CACHE_DIR out=$OUT_DIR"

if [[ -n "$LOCAL_SRC" ]]; then
  echo "[dapp-example] using local source: $LOCAL_SRC"
  rm -rf "$CACHE_DIR"
  mkdir -p "$CACHE_DIR"
  rsync -a --delete \
    --exclude node_modules --exclude dist --exclude .git \
    --exclude example/node_modules --exclude example/dist \
    "$LOCAL_SRC/" "$CACHE_DIR/"
else
  echo "[dapp-example] repo=$REPO_URL ref=$REF"
  if [[ ! -d "$CACHE_DIR/.git" ]]; then
    rm -rf "$CACHE_DIR"
    git clone --depth 1 --branch "$REF" "$REPO_URL" "$CACHE_DIR"
  else
    git -C "$CACHE_DIR" fetch --depth 1 origin "$REF"
    git -C "$CACHE_DIR" checkout -B "$REF" "origin/$REF"
  fi
fi

echo "[dapp-example] building SDK"
( cd "$CACHE_DIR" && npm install --no-audit --no-fund && npm run build )

echo "[dapp-example] building example with base=/dapp-example/"
(
  cd "$CACHE_DIR/example"
  npm install --no-audit --no-fund
  npx vite build --base=/dapp-example/
)

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"
cp -r "$CACHE_DIR/example/dist/." "$OUT_DIR/"

echo "[dapp-example] staged at $OUT_DIR"
