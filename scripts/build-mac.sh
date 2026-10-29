#!/bin/sh
# Builds the launcher for a Mac: bindings, frontend, then bin/omni (Apple Silicon). The macOS counterpart of build.ps1.
#   ./scripts/build-mac.sh                 build with the version "dev"
#   ./scripts/build-mac.sh 0.10.0          stamps the version shown in Settings
# Needs Go, Node, the Xcode command line tools (the Mac code is Objective-C through cgo) and wails3
# (go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.29). Packaging into Omni.app is milestone M4.
set -eu
cd "$(dirname "$0")/.."
version="${1:-dev}"

# The engine's source is embedded in the executable; compiled Python files must not go with it.
find sidecar/src -type d -name __pycache__ -prune -exec rm -rf {} +

(cd frontend && npm install --no-audit --no-fund)
wails3 generate bindings -clean=true -ts -i ./cmd/omni
(cd frontend && npm run build --silent)

mkdir -p bin
CGO_ENABLED=1 GOARCH=arm64 go build -tags production -trimpath -ldflags "-w -s -X main.version=$version" -o bin/omni ./cmd/omni
echo "built bin/omni"
