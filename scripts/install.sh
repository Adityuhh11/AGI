#!/bin/sh
# Installs Omni on a Mac with Apple Silicon:
#   curl -fsSL https://github.com/Adityuhh11/Omni/releases/latest/download/install.sh | sh
# It finds the latest release, downloads the zip, checks it against the SHA-256 GitHub published for it, puts Omni.app in ~/Applications
# and opens it. Nothing else: no sudo, no other file is touched. (A file fetched by curl is not marked as downloaded from the internet,
# so macOS does not stop it the way it stops one saved by a browser. docs/INSTALL-MAC.md has the browser route.)
# For another folder or version: OMNI_INSTALL_DIR=/Applications, or OMNI_API=https://api.github.com/repos/Adityuhh11/Omni/releases/tags/v0.10.0
# (OMNI_NO_OPEN=1 leaves it unopened; the three are also what the test, scripts/test-package-mac.sh, uses.)
set -eu

fail() { echo "install.sh: $*" >&2; exit 1; }

[ "$(uname -s)" = Darwin ] || fail "this is the Mac installer; Windows has its own (see the README)"
[ "$(sysctl -n hw.optional.arm64 2>/dev/null || echo 0)" = 1 ] || fail "Omni needs a Mac with Apple Silicon (M1 or later)"
[ "$(sw_vers -productVersion | cut -d. -f1)" -ge 13 ] || fail "Omni needs macOS 13 or later"

api="${OMNI_API:-https://api.github.com/repos/Adityuhh11/Omni/releases/latest}"
dir="${OMNI_INSTALL_DIR:-$HOME/Applications}"
mkdir -p "$dir"
work="$(mktemp -d "$dir/.omni-install.XXXXXX")" # inside the target folder, so the last step is a rename and not a copy
trap 'rm -rf "$work"' EXIT

echo "Looking for the latest release..."
curl -fsSL "$api" -o "$work/release.json" || fail "could not read $api"
# The release's file list is JSON, which a Mac has no tool for in the shell except this one (JavaScript for Automation): tag, address and digest of the zip.
found="$(osascript -l JavaScript -e '
ObjC.import("Foundation")
function run(argv) {
	var text = $.NSString.stringWithContentsOfFileEncodingError(argv[0], 4, null).js
	var release = JSON.parse(text)
	var zip = release.assets.filter(function (a) { return /^omni-.*-macos-arm64\.zip$/.test(a.name) })[0]
	return zip ? [release.tag_name, zip.browser_download_url, zip.digest || ""].join(" ") : ""
}' "$work/release.json")" || fail "could not read the release"
[ -n "$found" ] || fail "that release has no Mac download"
tag="${found%% *}"
rest="${found#* }"
url="${rest%% *}"
digest="${rest#* }"
case "$digest" in sha256:????????????????????????????????????????????????????????????????) ;; *) fail "GitHub shows no checksum for the download, so it cannot be checked" ;; esac

echo "Downloading Omni $tag..."
curl -fsSL "$url" -o "$work/omni.zip" || fail "could not download $url"
[ "sha256:$(shasum -a 256 "$work/omni.zip" | cut -d' ' -f1)" = "$digest" ] || fail "the download does not match the checksum GitHub published; nothing was installed"

ditto -x -k "$work/omni.zip" "$work/new"
[ -d "$work/new/Omni.app" ] || fail "the download does not hold Omni.app"
codesign --verify --deep --strict "$work/new/Omni.app" || fail "the app's signature is broken; nothing was installed"

# An older Omni that is running is asked to stop first, so the new one is not started beside it.
if pkill -TERM -f "$dir/Omni.app/Contents/MacOS/omni" 2>/dev/null; then sleep 2; fi
rm -rf "$dir/Omni.app"
mv "$work/new/Omni.app" "$dir/Omni.app"
echo "Installed $dir/Omni.app"

[ -n "${OMNI_NO_OPEN:-}" ] || open "$dir/Omni.app"
echo "Omni is in the menu bar. Its setup guide opens on the first start."
