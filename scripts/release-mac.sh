#!/bin/sh
# Builds the launcher and packs it for someone else, the macOS counterpart of release.ps1: dist/omni-<version>-macos-arm64.zip holds
# Omni.app, and dist/install.sh is the script people run to fetch it. A published release gives its number, which is also its tag without
# the "v" (the update check compares it); a test build is named by the date and the commit (with -dirty if the tree has changes).
#   ./scripts/release-mac.sh 0.10.0     for the release v0.10.0
#   ./scripts/release-mac.sh            for a test build
# The bundle is signed ad hoc (no Apple account, decision 0010): enough for an Apple Silicon Mac to run it, nothing for Gatekeeper to trust.
set -eu
cd "$(dirname "$0")/.."

if [ -n "${1:-}" ]; then
	version="$1"
	printf %s "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$' || { echo "version must look like 0.2.0, got $version" >&2; exit 1; }
else
	dirty=""
	[ -z "$(git status --porcelain)" ] || dirty="-dirty"
	version="$(date +%Y.%m.%d)-$(git rev-parse --short HEAD)$dirty"
fi
# The bundle's own version is three numbers (macOS reads nothing else); the program reports the whole of $version.
plistversion="$(printf %s "$version" | sed -n 's/^\([0-9]*\.[0-9]*\.[0-9]*\).*/\1/p')"

./scripts/build-mac.sh "$version"

app="dist/Omni.app"
rm -rf dist
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp bin/omni "$app/Contents/MacOS/omni"
cp build/darwin/omni.icns "$app/Contents/Resources/omni.icns"
sed "s/{VERSION}/${plistversion:-0.0.0}/g" build/darwin/Info.plist > "$app/Contents/Info.plist"
plutil -lint "$app/Contents/Info.plist" >/dev/null

# The identifier is the bundle id, so what macOS remembers about the app does not hang on how it was signed.
codesign --force --deep --sign - --identifier io.github.adityuhh11.omni "$app"
codesign --verify --deep --strict "$app"

# ditto, not zip: it keeps the bundle's signature, links and executable bits as they are.
zip="dist/omni-$version-macos-arm64.zip"
ditto -c -k --keepParent "$app" "$zip"
cp scripts/install.sh dist/install.sh
echo "built $zip ($(du -h "$zip" | cut -f1))"
echo "sha256 $(shasum -a 256 "$zip" | cut -d' ' -f1)"
