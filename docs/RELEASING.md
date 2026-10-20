# Releasing

## Version numbers

`MAJOR.MINOR.PATCH` ([SemVer](https://semver.org)), tag `vMAJOR.MINOR.PATCH`, release title `AGI MAJOR.MINOR.PATCH`. The number
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
2. In a clean worktree of `origin/main`: `scripts\release.ps1 -Version X.Y.Z`. The zip is `dist\agi-X.Y.Z-windows-x64.zip`.
3. Unzip it somewhere new and smoke-test it with a clean profile (`APPDATA` and `LOCALAPPDATA` pointed at an empty folder):
   it starts, the hotkey opens the bar, Settings opens on About and says `X.Y.Z`.
4. `gh release create vX.Y.Z <zip> --target <sha of main> --title "AGI X.Y.Z" --notes-file <notes>`. The notes say what a person
   gets, not what changed in the code, and name anything that does not work yet.
5. Download the published asset and compare its SHA-256 with the file that was uploaded. The one-click update installs only a
   file that matches the digest GitHub shows for the asset, so a release without a digest cannot be installed from the program.
6. Bring `dev` up to date with `main`.
