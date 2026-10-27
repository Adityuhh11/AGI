# Releasing

## Version numbers

`MAJOR.MINOR.PATCH` ([SemVer](https://semver.org)), tag `vMAJOR.MINOR.PATCH`, release title `Omni MAJOR.MINOR.PATCH`. The number
without the `v` is what `scripts/release.ps1 -Version` takes, what the program reports, and what the update check compares.

- **While MAJOR is 0** (now): a new feature or a change in how something behaves bumps MINOR (`0.3.0` to `0.4.0`); a fix that
  adds nothing bumps PATCH (`0.4.0` to `0.4.1`). Nothing is promised to stay compatible between MINORs.
- **1.0.0** is cut when the product is something a stranger could install and rely on: signed or otherwise trusted to run, and an
  update path that has crossed more than one release in practice. From then on MAJOR is for a break in what a person relies on
  (a config or data file that no longer loads, a changed hotkey default), MINOR for features, PATCH for fixes.
- No calendar versions, no build numbers. A pre-release is `X.Y.Z-rc.1` and is marked as a pre-release on GitHub; the update
  check treats it as older than `X.Y.Z` and the page offers only the latest non-pre-release.
- Test builds (`scripts/release.ps1` with no `-Version`) are named by date and commit and are not releases.

## Cutting a release

1. `dev` is merged into `main` by pull request (the owner merges), then `main` is the only source of a release.
2. Tag the merge commit and push the tag: `git tag vX.Y.Z <sha of main>` and `git push origin vX.Y.Z`. The Release workflow
   (`.github/workflows/release.yml`) refuses a tag that is not on `main`, builds the zip and the installer on a clean Windows
   machine with `scripts\release.ps1`, runs the installer for real (`scripts\test-installer.ps1`), attaches a build-provenance
   attestation, and makes a **draft** release `Omni X.Y.Z` with both files, checking that GitHub's digest of each is the one built.
   By hand (`gh workflow run release.yml`) it only uploads the files as an artifact. On a machine with Inno Setup,
   `scripts\release.ps1 -Version X.Y.Z` makes the same two files in `dist\` (the zip alone if the compiler is missing).
3. Smoke-test the files from the draft with a clean profile (`APPDATA` and `LOCALAPPDATA` pointed at an empty folder, and `OMNI_INSTANCE=x`
   to run beside the launcher in use): it starts, the setup guide opens, Settings > About says `X.Y.Z`.
4. Write the notes in the draft (they say what a person gets, not what changed in the code, and name anything that does not work
   yet) and publish it.
5. Download the published zip and compare its SHA-256 with the digest GitHub shows. The one-click update installs only a file that
   matches that digest, so a release without one cannot be installed from the program. (The update looks for the zip by name; the
   installer is for people installing, and is never fetched by the program.)
6. Bring `dev` up to date with `main`.
