# Installing Omni on a Mac

For a Mac with Apple Silicon (M1 or later) and macOS 13 or later. Omni is not signed by Apple (that costs $99 a year; the reasons and the plan are in
[decision 0010](decisions/0010-macos-distribution-without-apple-account.md)), so macOS treats a download from a browser with suspicion. The first route below avoids that.
Nothing on this page has been tried on a real Mac yet; the wording of the macOS dialogs is from Apple's documentation and may differ a little.

## 1. One line in Terminal (recommended)

Open **Terminal** (Spotlight: type "Terminal") and paste:

```sh
curl -fsSL https://github.com/Adityuhh11/Omni/releases/latest/download/install.sh | sh
```

It finds the latest release, downloads `Omni.app`, checks it against the SHA-256 GitHub published for it, puts it in `~/Applications` and opens it. It does nothing else:
no `sudo`, no other file is touched. To read it first, run the `curl` part alone: it prints the script. A file downloaded this way is not marked as "from the internet", so
macOS does not ask whether you trust it.

Another folder: `curl ... | OMNI_INSTALL_DIR=/Applications sh`. Running it again updates Omni.

## 2. Download in the browser

1. Download `omni-<version>-macos-arm64.zip` from the [latest release](https://github.com/Adityuhh11/Omni/releases/latest), open it, and drag **Omni.app** to Applications.
2. Double-click it. macOS says it cannot be opened because it is from an unidentified developer.
3. Open **System Settings > Privacy & Security**, scroll down to the message about Omni and press **Open Anyway**, then confirm with your password. (Since macOS 15
   the old right-click > Open shortcut no longer works.) It asks only once.

Or, in Terminal, remove the mark the browser put on it: `xattr -dr com.apple.quarantine /Applications/Omni.app`.

## First start

Omni has no Dock icon; it lives in the menu bar (its icon is on the right, by the clock), and the setup guide opens. Press **⌥⇧Space** for the bar. Change the key in Settings > General.
macOS will ask for three things, each only when it is needed:

| Asks | When | Say yes to |
|---|---|---|
| **Accessibility** | The first time you paste a clipboard item or snippet into another app | Let Omni press ⌘V for you. Without it Omni copies and tells you to press ⌘V. |
| **Files and Folders** (Documents, Desktop, Downloads, a drive) | When you add that folder to file search | Let Omni read it. A folder you refuse shows "needs permission" with a button to the right page of System Settings. |
| **Login Items** | When you switch on "Start when I sign in" | A notification from macOS, nothing to do. |

## Updating

There is no one-click update on a Mac yet. Run the one-line install again (or delete `Omni.app` and download the new one). Because the app is not signed with a stable
identity, macOS may forget the Accessibility permission after an update (the switch still looks on): in **Privacy & Security > Accessibility** remove Omni with the **−** button, and Omni asks again at the next paste.

## Removing it

Quit Omni from the menu bar and delete `Omni.app`. What Omni saved (settings, the engine and models, the index, chats, clipboard history) is in
`~/Library/Application Support/Omni`, and the log in `~/Library/Logs/Omni`; delete those too if you want everything gone. Your own files are never touched.

## Checking the download

`gh attestation verify omni-<version>-macos-arm64.zip --repo Adityuhh11/Omni` proves the file was built from this repository by its release workflow.

## Building it yourself

`./scripts/build-mac.sh` builds `bin/omni` (needs Go, Node, the Xcode command line tools and wails3); `./scripts/release-mac.sh` makes `dist/Omni.app`, the zip and `install.sh`.
