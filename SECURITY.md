# Security

## Reporting a problem

Please report a vulnerability privately: on this repository's **Security** tab choose **Report a vulnerability**
(https://github.com/Adityuhh11/AGI/security/advisories/new). Do not open a public issue for it. Include the AGI version (Settings > About),
what you did, and what you could do that you should not have been able to.

## What is supported

The latest release. AGI is early (0.x): a fix ships as a new release and the in-app update offers it.

## What matters most here

AGI runs on a person's own computer and reads their files, so the things that must not break are:

- **Nothing is sent anywhere unasked.** The only network calls are the ones in the README's privacy table; no telemetry.
- **The memory engine's local API is private to the launcher.** It listens on `127.0.0.1` only, needs a random per-run token on every route, and
  rejects foreign `Host` headers.
- **Files are opened only inside folders the person indexes**, and secrets (`.env*`, keys and similar) are never read.
- **Downloads are pinned.** The engine's components are checked by SHA-256, and the one-click update installs only a file that matches the digest
  GitHub published for the release.

A way around any of these is in scope. See `docs/ARCHITECTURE.md` (invariants) and `docs/api.md`.
