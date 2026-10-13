#!/usr/bin/env bash
# Build the single static binary ./stage with the pinned toolchain.
set -euo pipefail
cd "$(dirname "$0")"
scripts/fetch-go.sh
export PATH="$PWD/.toolchain/go/bin:$PATH" GOTOOLCHAIN=local
CGO_ENABLED=0 go build -trimpath -o stage .
