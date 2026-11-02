#!/usr/bin/env bash
set -euo pipefail

OUTPUT_PATH="${CPA_BUILD_OUTPUT_PATH:-/tmp/quota-activation.so}"

echo "Building quota-activation plugin..."
echo "Target output: ${OUTPUT_PATH}"

CGO_ENABLED=1 go build -buildmode=c-shared -o "${OUTPUT_PATH}" .

echo "Build succeeded."
