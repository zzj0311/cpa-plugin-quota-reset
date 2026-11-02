#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PLUGIN_ID="${PLUGIN_ID:-quota-reset}"
mkdir -p "$ROOT/dist"
docker run --rm \
  -v "$ROOT:/src" -w /src \
  -e "PLUGIN_ID=${PLUGIN_ID}" \
  golang:1.25-bookworm \
  bash -lc 'set -euo pipefail
    export PATH=/usr/local/go/bin:$PATH
    apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq gcc libc6-dev >/dev/null
    CGO_ENABLED=1 go build -buildvcs=false -buildmode=c-shared -o "dist/${PLUGIN_ID}.so" .
    ls -la "dist/${PLUGIN_ID}.so"'
echo "Built $ROOT/dist/${PLUGIN_ID}.so"
