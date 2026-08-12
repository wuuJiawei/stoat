#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

unformatted="$(gofmt -l .)"
if [[ -n "$unformatted" ]]; then
    echo "Go files require formatting:"
    echo "$unformatted"
    exit 1
fi

go vet ./...
go test -race ./...
shellcheck -x scripts/*.sh
bash scripts/install_test.sh
go build ./...
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /tmp/stoat-check-darwin-arm64 ./cmd/stoat
rm -f /tmp/stoat-check-darwin-arm64
