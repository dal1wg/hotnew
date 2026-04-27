#!/usr/bin/env bash
set -euo pipefail

OUTPUT="${1:-bin/hotnew}"
GOOS_VALUE="${GOOS:-linux}"
GOARCH_VALUE="${GOARCH:-amd64}"

echo "[build] GOOS=${GOOS_VALUE} GOARCH=${GOARCH_VALUE}"
mkdir -p "$(dirname "$OUTPUT")"
GOOS="${GOOS_VALUE}" GOARCH="${GOARCH_VALUE}" go build -o "$OUTPUT" ./cmd/hotnew

echo "[build] output => ${OUTPUT}"
