#!/bin/sh
# Meets the package the way a person does, after scripts/release-mac.sh has built it: install.sh installs it (from a release file list made here,
# as GitHub's would be, so the real script runs against the real zip), the bundle is what it should be, a download that does not match its
# checksum is refused, and the installed program starts and stays up in the bare environment Finder gives a program. The macOS counterpart of
# test-installer.ps1; the CI job and the Release workflow both run it.
set -eu
cd "$(dirname "$0")/.."

zip="$(ls dist/omni-*-macos-arm64.zip)"
work="$(mktemp -d)"
trap 'kill "$pid" 2>/dev/null || true; rm -rf "$work"' EXIT
pid=""

release() { # $1 = the digest to publish
	printf '{"tag_name":"v0.0.0","assets":[{"name":"%s","browser_download_url":"file://%s/%s","digest":"%s"}]}' "$(basename "$zip")" "$PWD" "$zip" "$1" > "$work/release.json"
}
run_install() { OMNI_API="file://$work/release.json" OMNI_INSTALL_DIR="$work/apps" OMNI_NO_OPEN=1 sh scripts/install.sh; }

echo "--- a download that does not match its checksum is refused"
release "sha256:0000000000000000000000000000000000000000000000000000000000000000"
if run_install; then echo "install.sh installed a download with the wrong checksum"; exit 1; fi
[ ! -e "$work/apps/Omni.app" ] || { echo "the refused download was installed anyway"; exit 1; }

echo "--- the right one is installed"
release "sha256:$(shasum -a 256 "$zip" | cut -d' ' -f1)"
run_install
app="$work/apps/Omni.app"
run_install # again, over the first: an update

echo "--- the bundle"
plutil -lint "$app/Contents/Info.plist"
[ "$(plutil -extract CFBundleIdentifier raw "$app/Contents/Info.plist")" = io.github.adityuhh11.omni ]
[ "$(plutil -extract LSUIElement raw "$app/Contents/Info.plist")" = true ]
[ -f "$app/Contents/Resources/omni.icns" ]
lipo -archs "$app/Contents/MacOS/omni" | grep -qx arm64
codesign --verify --deep --strict "$app"

echo "--- it starts and stays up, as Finder would start it"
home="$work/home"
mkdir -p "$home"
env -i HOME="$home" PATH=/usr/bin:/bin:/usr/sbin:/sbin OMNI_INSTANCE=pkg "$app/Contents/MacOS/omni" &
pid=$!
sleep 10
log="$home/Library/Logs/Omni/omni.log"
kill -0 "$pid" || { echo "the installed program is not running"; cat "$log" || true; exit 1; }
grep -q "omni starting" "$log"
echo "ok"
