# Distribution: getting Omni installed without Windows crying wolf

Written 2026-10-24 from current vendor pages; the route is a recommendation, not a decision yet. Facts marked *(verify)* came
from third-party guides or could not be confirmed on the vendor's own page; check them before spending money or time.

## What goes wrong today

A download of `omni.exe` meets two separate systems:

| System | What it does | Why Omni trips it |
|---|---|---|
| **SmartScreen** (reputation) | Warns "Windows protected your PC" on a file downloaded from the internet that is unsigned or whose signer has little reputation. Click-through: More info, Run anyway. | The exe is unsigned and new. Since 2024 even an EV certificate does not skip it; reputation builds with downloads. |
| **Microsoft Defender antivirus** (machine-learning verdicts such as `Trojan:Win32/Wacatac.B!ml`) | Scores a file on how it looks and what it does; unsigned, rarely seen files that behave "like a dropper" are quarantined. A `!ml` verdict is a guess, not proof. | See below. |

The behaviours that read as suspicious, all legitimate and all required by the product: it registers a global hotkey, watches
the clipboard, types Ctrl+V into other programs (`SendInput`), writes the per-user `Run` key, downloads `uv.exe` and
`llama-server.exe` and runs them, and replaces and restarts its own exe to update. Go binaries are also over-represented in
false positives. None of this is fixable by removing features; it is fixed by **identity (a signature), history (reputation) and
transparency (reproducible builds, hashes, scans)**.

## Route, in order

### 1. Now, free, every release

- **Submit each release zip to Microsoft** as a software developer at the Security Intelligence file-submission portal
  (`microsoft.com/wdsi/filesubmission`) *before* announcing it. Users report verdicts being cleared within hours to a day *(verify)*.
  After each release, download the published zip back and submit that exact file.
- **Scan with VirusTotal** and link the result from the release notes, so a visitor can see 1 vendor out of 70 is a guess and
  not 20 of 70. Do not "fix" detections by packing or obfuscating the exe: it makes things worse.
- **Build releases in public CI** (GitHub Actions on a tag), upload the files from the runner and attach a build-provenance
  attestation (`actions/attest-build-provenance`), so "this binary came from this commit" is checkable. This is also what code
  signing services require. Done in U4 (`.github/workflows/release.yml`; `gh attestation verify <file> --repo Adityuhh11/Omni`).
- Keep what is already right: SHA-256 of the asset on the release page, every downloaded component pinned by SHA-256, no packer,
  no telemetry, README table of every network call.
- Embed Windows version information, an icon and a manifest in `omni.exe` (company, product, description, version; the manifest says it
  runs as the user and is made for Windows 10 and 11). Unlabelled executables score worse. Done in U4: `scripts/build.ps1` generates
  the resource from the version being built; before it, `bin\omni.exe` had none. It is still `NotSigned`.

### 2. Sign the exe (the single biggest improvement)

| Option | Cost | Fit for Omni |
|---|---|---|
| **SignPath Foundation** (free signing for open-source projects) | free | Best fit once the repo is open source. Needs: an OSI-approved licence with no commercial dual-licensing, a public repository, an actively maintained project already released in the form to be signed, a fully automated build in CI, and a code-signing policy page on the project's site. The certificate is issued to the SignPath Foundation, so *it* is the publisher shown in Windows, and you follow its rules (violations can revoke). |
| **Azure Artifact Signing** (formerly Trusted Signing) | $9.99 / month (5,000 signatures) | Cheap and CI-friendly, but individuals appear to be limited to the **USA and Canada**, organisations to USA, Canada, EU and UK *(verify on Microsoft's page; it is said to be expanding)*. Whether it is open to the owner depends on where they live. |
| **OV code-signing certificate** from a CA | about $150 to $300 / year *(verify)* | Works anywhere with identity checks; the private key must live on a hardware token or cloud HSM, so CI needs a signing service. |
| **EV certificate** | more | No longer bypasses SmartScreen (Microsoft's own docs say so). Not worth it for us. |

Whichever is used, SmartScreen reputation still accrues over weeks of downloads; the first releases may still show the warning, and
the README keeps the "More info, Run anyway" instructions until they stop.

**Recommendation:** pick a licence, then apply to SignPath Foundation; if the owner is in the USA or Canada, Azure Artifact
Signing is the quicker alternative while that is pending. Sign `omni.exe` inside the zip in CI. One thing to rehearse: the
one-click updater replaces `omni.exe` from a zip, so the signed exe, not the zip, is what carries the signature (the updater already
verifies the zip's SHA-256).

### 3. Other ways to install

| Channel | What it gives | Catch |
|---|---|---|
| **winget** (manifest in `microsoft/winget-pkgs`) | `winget install` for users; the package is validated and scanned by Microsoft before it is accepted; trust by association. | Needs a stable installer URL and hash per release (we have both). A zip with a portable exe is a supported shape *(verify the manifest fields)*. Re-submit per release, which a CI step can do. |
| **Scoop bucket** | Developer audience; a manifest in our own repo. | Small audience; fine as a cheap extra. |
| **Microsoft Store (MSIX)** | Store-signed, no SmartScreen warning, and **individual developer registration is now free** (Microsoft, September 2025). | Largest effort and the most risk: a packaged app has a different identity and file-system view (our `%LOCALAPPDATA%\Omni\engine`, the `Run` key and the self-updater all behave differently or are disallowed); an app that downloads and runs executables may fail certification. Treat as a later experiment, not the plan. |

## macOS

A Mac asks the same two questions in other words: Gatekeeper (is this signed by an Apple-registered developer?) and the permission prompts. The route, which costs nothing, is in
[decision 0010](decisions/0010-macos-distribution-without-apple-account.md): `Omni.app` signed ad hoc (Apple Silicon will not run a binary with no signature, and an ad-hoc one is free
but gives Gatekeeper nothing to trust), a one-line `install.sh` whose download is not quarantined and so is not stopped by Gatekeeper, and the zip with the Open Anyway steps for
browser downloads ([INSTALL-MAC.md](INSTALL-MAC.md)). Developer ID, notarization and a Homebrew cask wait for the $99 Apple Developer Program; they change `scripts/release-mac.sh`, not the app.
Each release's Mac files are built and attested by the same Release workflow.

## Decisions the owner has to make

1. **A licence.** Done: MIT (`LICENSE`), which SignPath's OSI requirement accepts.
2. **The publisher name Windows will show** (a person, an organisation, or "SignPath Foundation") and where the owner lives, which
   decides whether Azure Artifact Signing is open to them.
3. **The project name.** "Omni" is hard to search for and hard to defend as a trademark; changing it is cheapest before the first
   public announcement.

## What this document does not cover

Whether a given release is flagged on a given day cannot be tested in advance; the only reliable check is the portal verdict and a
clean Windows 11 machine. After the first signed release, record what a fresh download shows in `docs/eval/`.

## Sources

- Windows code-signing options: https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options
- Azure Artifact Signing pricing: https://azure.microsoft.com/pricing/details/artifact-signing/
- SignPath Foundation, open-source terms: https://signpath.org/terms and https://about.signpath.io/product/open-source
- Free individual Store registration: https://blogs.windows.com/windowsdeveloper/2025/09/10/free-developer-registration-for-individual-developers-on-microsoft-store/
