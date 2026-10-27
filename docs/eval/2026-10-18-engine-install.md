# Installing file search from the app (packaging block E, 2026-10-18)

The real thing, not a fake server: `bin\omni.exe` built by `scripts\build.ps1` (the engine's source is embedded, 13 MB exe), started in a
brand-new profile (`%APPDATA%` and `%LOCALAPPDATA%` empty, a config with `roots = ['...\Sample Data']` and file search off), driven with
real key presses and clicks: hotkey, `Ctrl+,`, File search, **Download and turn on**. Network: github.com, pypi.org, files.pythonhosted.org,
huggingface.co.

| Check | Result |
|---|---|
| Press the button to "file search is installed" (uv, engine source, Python 3.12 and libraries, llama.cpp, two models, check) | **2 min 6 s** on the owner's connection |
| Size on disk afterwards | 1.1 GB: models 825 MB, llama.cpp 88 MB, venv 86 MB, Python 69 MB, uv 32 MB, engine source 1 MB. About 0.9 GB was downloaded (the page says "about 0.9 GB") |
| Progress as shown | "Step 1 of 6: Getting the package tool, 0.1 of 15 MB"; "Step 3 of 6: Installing Python and its libraries" with uv's own line ("Downloading pillow (6.9MiB)"); a bar for downloads |
| After the install | file search switched on by itself, the engine started, the folder was indexed |
| Search from the bar (`how do I refresh the jwt token`) | `jwt.js`, `user.route.js`, `user.controller.js`, `auth.middleware.js`, `README.md`...: the right files, with the matching lines |
| An engine that is out of date (this build carries newer engine code) | the page says so and offers **Update**; the update, with the engine running, took about 5 s (it stops the engine, replaces its source, syncs, checks, starts it) |
| Resume after a failed update (manifest gone, every download still on disk) | 6 s: the downloads are checked, not fetched again |
| Killing the launcher from outside | the engine, its Python child and `llama-server` all end (the launcher's job object and the engine's own) |

## Found by this run, fixed

1. **Search failed with `401 Invalid API Key`.** `llama-server` closes an idle keep-alive connection after a few seconds. The embedder took that
   for a dead server and restarted it, which gives a new API key, but the request headers had been built once before the retry, so the retry
   carried the old key. Now: a failure first reconnects, only a second failure restarts the server, and the headers are built on every try
   (tests: `test_a_connection_the_server_closed_while_idle...`, `test_a_server_that_is_gone_is_started_again_and_the_new_key_is_used`).
   The unit tests with the fake server had not caught it because the fake never closed an idle connection nor changed its key.
2. **The first update failed with "being used by another process"** while clearing the engine's source folder: the engine had just been killed
   and Windows holds its files for a moment. Removals now retry for up to five seconds (`removeAll`, tested with a file that is held open and then
   released).

Not run: cancelling a real download and continuing it, an install with no network, a machine without the discrete GPU's Vulkan driver, and
the friend's machine.
