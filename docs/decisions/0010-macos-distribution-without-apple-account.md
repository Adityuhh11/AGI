# 0010: macOS without a paid Apple account

Date: 2026-10-29. Status: proposed (owner to confirm).

## Decision

Ship `Omni.app` for **Apple Silicon only**, **ad-hoc signed**, as a zip on the GitHub release, plus an `install.sh` that downloads and verifies it from Terminal. No
Developer ID, no notarization, no Mac App Store, no Homebrew cask until the owner pays for the Apple Developer Program ($99 a year). Move to a stable self-signed identity in M5 so the Accessibility permission survives updates.

## Why

- Apple Silicon refuses an unsigned binary; an ad-hoc signature is free and enough to run.
- A file fetched by `curl` is not quarantined, so Gatekeeper never inspects it: `install.sh` gives a warning-free install for the cost of one pasted line.
- A browser download is quarantined and shows "cannot be opened"; the person must use Privacy & Security → Open Anyway once. Acceptable for a friend, not for the public, so the public path waits for notarization.
- Homebrew announced that casks failing Gatekeeper would be dropped from 1 September 2026, so a cask is closed to us until we are notarized.
- Ad-hoc signed apps are identified by the hash of their bytes, so an update looks like a different app and macOS silently drops the Accessibility grant. A stable self-signed certificate keeps the grant (reported by other developers; to be verified on the friend's Mac).

- The bar, the cards and the timer are clear windows, and a Mac web view can only be made clear through an undocumented WebKit key that Wails uses
  only when built with `-tags private_mac_apis` (found in the first screenshot of the bar: an opaque plate behind it). The tag is on in `scripts/build-mac.sh`. It rules out the Mac App Store,
  which this decision rules out already; Developer ID and notarization are not affected.

## Alternatives rejected

| Option | Why not |
|---|---|
| Pay for the Apple account now | Not needed to let a friend run it; later and without redesign. |
| Intel (x86_64) build | Apple and Homebrew are ending Intel support; doubles the CI and the test matrix for nobody known. |
| `xattr -cr` as the only instruction | Works but teaches a security habit we should not teach; `install.sh` avoids the need. |
| Disable Gatekeeper system-wide (`spctl --master-disable`) | Never. |

## Consequences

First launch from a browser download needs the Open Anyway steps (documented in `docs/INSTALL-MAC.md`). Signing, notarization and the cask become a later change to `scripts/release-mac.sh`, not to the app.
