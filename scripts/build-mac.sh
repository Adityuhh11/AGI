#!/bin/sh
# Builds the launcher for a Mac: bindings, frontend, then bin/omni (Apple Silicon). The macOS counterpart of build.ps1.
#   ./scripts/build-mac.sh                 build with the version "dev"
#   ./scripts/build-mac.sh 0.10.0          stamps the version shown in Settings
# private_mac_apis: a web view only draws without a background (the bar, the cards and the timer are clear windows with the page drawing
# its own edge) through an undocumented WebKit key, which Wails uses only with this tag; without it the page sits on an opaque plate.
# The Mac App Store would refuse such a build, which decision 0010 already rules out.
# Needs Go, Node, the Xcode command line tools (the Mac code is Objective-C through cgo) and wails3
# (go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.29). release-mac.sh packs the result into Omni.app.
set -eu
# The program is for macOS 13 and later (the login item needs it); without this the linker is told the version of the machine it runs on.
export MACOSX_DEPLOYMENT_TARGET=13.0
cd "$(dirname "$0")/.."
version="${1:-dev}"

# The engine's source is embedded in the executable; compiled Python files must not go with it.
find sidecar/src -type d -name __pycache__ -prune -exec rm -rf {} +

(cd frontend && npm install --no-audit --no-fund)
wails3 generate bindings -clean=true -ts -i ./cmd/omni
(cd frontend && npm run build --silent)

mkdir -p bin
CGO_ENABLED=1 GOARCH=arm64 go build -tags production,private_mac_apis -trimpath -ldflags "-w -s -X main.version=$version" -o bin/omni ./cmd/omni
echo "built bin/omni"
