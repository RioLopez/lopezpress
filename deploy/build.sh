#!/usr/bin/env bash
# Minimal cross-compile script for the echo-content-api.
# Produces a static Linux amd64 binary (no CGO) suitable for a minimal
# base image.
set -euo pipefail

cd "$(dirname "$0")/.."

mkdir -p bin

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
	-ldflags="-s -w" \
	-o bin/echo-content-api \
	./cmd/server

echo "built: $(pwd)/bin/echo-content-api"
