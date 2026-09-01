#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

BIN_DIR="bin"
BIN="$BIN_DIR/runbook"

mkdir -p "$BIN_DIR"

echo "==> building $BIN"
go build -o "$BIN" ./cmd/runbook

echo "==> running $BIN"
exec "./$BIN" "$@"
