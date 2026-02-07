#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "=== Step 1/5: go build ==="
go build ./...

echo "=== Step 2/5: go vet ==="
go vet ./...

echo "=== Step 3/5: golangci-lint ==="
golangci-lint run ./...

echo "=== Step 4/5: go test ==="
go test ./... -count=1 -timeout 120s

echo "=== Step 5/5: go test -race ==="
go test ./... -race -count=1 -timeout 120s

echo ""
echo "All checks passed."
