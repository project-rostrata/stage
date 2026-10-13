#!/usr/bin/env bash
# Vet and test everything with the pinned toolchain.
set -euo pipefail
cd "$(dirname "$0")"
scripts/fetch-go.sh
export PATH="$PWD/.toolchain/go/bin:$PATH" GOTOOLCHAIN=local
# gofmt is given the module's own files: given a directory, it would also walk .toolchain/.
unformatted="$(gofmt -l $(go list -f '{{range .GoFiles}}{{$.Dir}}/{{.}} {{end}}{{range .TestGoFiles}}{{$.Dir}}/{{.}} {{end}}' ./...))"
test -z "$unformatted" || { echo "gofmt needed:"; echo "$unformatted"; exit 1; }
go vet ./...
CGO_ENABLED=0 go test ./...
