# Launching apps: spike S1 (2026-10-17)

Question: can the launcher start Start Menu shortcuts and Store apps with the `ShellExecute` call it already uses for files,
and what does it cost or do on failure? A throwaway Go program on this machine (Windows 11, no elevation).

| Target | How | Result |
|---|---|---|
| Calculator (Store) | `shell:AppsFolder\Microsoft.WindowsCalculator_8wekyb3d8bbwe!App` | works; call 419 ms (cold), window after 628 ms |
| Settings (Store) | `shell:AppsFolder\windows.immersivecontrolpanel_cw5n1h2txyewy!microsoft.windows.immersivecontrolpanel` | works; call 38 ms, window after 93 ms |
| 7-Zip File Manager | the Start Menu `.lnk` | works, the window opens, **but the call took 2.1 s** |
| Shortcut whose target is gone | the `.lnk` | an error, "The operation was canceled by the user" (Windows' own dialog territory) |
| Unknown Store app id | `shell:AppsFolder\No.Such.App_xyz!App` | clean error, "The system cannot find the file specified" |

What it means for the design:

- Both kinds start with the same call. The AppUserModelID in `Get-StartApps` is exactly what `shell:AppsFolder\` needs.
- **`ShellExecute` can block for seconds** on a classic shortcut (2.1 s here), so launching runs on its own goroutine and
  the panel closes without waiting for the call; the call's duration is not the launch time.
- A dead shortcut gives an unhelpful error and may raise Windows' own dialog. The index stores each shortcut's target
  (read from the `.lnk`), and the launcher checks the target exists before calling, so a dead entry is dropped with a
  one-line note and Windows never shows a dialog. Store apps are checked against the current app list instead.
- Not tested: the administrator verb (`runas` raises a UAC prompt that needs a human); it is checked at the owner's
  review of block E.
