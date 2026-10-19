AGI {VERSION}
A launcher for Windows: press a hotkey, type, and open an app, search the web, jump to a quicklink,
or (optionally) find your own files by what they say. Everything runs on this computer.

STARTING IT

  1. Unzip the folder anywhere and run agi.exe.
     Windows may say "Windows protected your PC", because the program is not signed yet.
     Click "More info", then "Run anyway".
  2. There is no window of its own. The program sits in the notification area (the little arrow
     by the clock). The first time it starts it opens Settings so you can see the hotkey.
  3. Press Alt+Shift+Space to open the bar. Settings > General changes the hotkey, and says
     so if another program already owns the one you pick.
  4. Settings > General > "Start when I sign in" makes it start with Windows. It remembers where
     agi.exe is; if you move the file, switch that off and on again.

WHAT TO TRY

  - Type a few letters of an app (vsc, chr, ppt) and press Enter. An app that is already open
    is brought to the front instead of being started a second time.
  - Tab switches what the bar covers: everything, apps, files, the web. "?" or "web:" at the start
    searches the web; Alt+Enter does it from any row. It opens in the browser set in Settings.
  - Quicklinks: Settings > Quicklinks. "gh rust lifetimes" opens that search on GitHub.
  - Ctrl+T on a row tags it. "#work" lists everything tagged work.
  - Ctrl+K opens what can be done with the selected row. Ctrl+, opens Settings.

FILE SEARCH (optional, off until you turn it on)

  Finds files by what they contain, not by their names: code, PDF pages, notes, and pictures by
  what they show ("the diagram with the three boxes"). It is the part of AGI that needs the most
  from your computer, so it is a choice.

  Settings > File search > "Download and turn on".
  - It downloads about 0.9 GB once, from github.com, pypi.org, files.pythonhosted.org and
    huggingface.co, and says so before you press the button. Afterwards AGI makes no network
    calls for it. It took about two minutes on a fast connection.
  - It runs on the graphics card (NVIDIA, AMD or Intel Arc, through Vulkan) and falls back to the
    processor, which is much slower, if there is none or its driver does not support Vulkan.
  - Add the folders you want searched, one at a time (Settings > File search > Add folder).
    Nothing is read until you do.
  - Indexing runs in the background. Pick how hard it works: Fast, Balanced (the default, about
    twice as long) or Gentle (about six times as long, the computer stays free). Pause and Resume
    are there too, and in the menu of the notification-area icon. Searching works throughout, on
    what is indexed so far.
  - "Turn off" stops it and keeps the files. "Delete downloaded files" removes the engine and the
    models (about 1.1 GB on disk) and keeps the index, which works again after turning it on.
  - A new version of AGI may come with a newer search engine; Settings offers an Update button.

WHERE THINGS ARE

  Settings file   %APPDATA%\AGI\config.toml
  Log             %LOCALAPPDATA%\AGI\logs\agi.log
  Search engine   %LOCALAPPDATA%\AGI\engine      (only after you turn file search on)
  Index           %LOCALAPPDATA%\AGI\memory      (what file search has indexed)
  Quit            right-click the icon in the notification area, then Quit
  Remove it all   quit it, delete this folder, then delete the folders above.
  The program has no telemetry. Besides the downloads of file search it only touches the network
  when you open a web search or a link (your browser does that), and when you ask about updates:
  Settings > About > "Check for updates", or "Check once a day" if you turn it on (off by default).
  That request goes to api.github.com, asks for the latest version number and sends nothing else.
  A new version is downloaded by you from its release page; replace agi.exe with the new one.

NOT HERE YET

  Clipboard history, snippets, window management, system commands, searching Office files and
  videos by their content (they are found by name only).

WHAT I WOULD LIKE TO KNOW

  1. Did it start without trouble? Did the hotkey work, or does something else have it?
  2. Next to what you use today: what do you reach for that is missing, slower, or in the wrong place?
  3. If you turned file search on: how long did the download take, which folders did you add, and
     did indexing get in your way (heat, fans, a slow computer, a game stuttering)? Did the speeds
     feel right? Were the results you expected first?
  4. Anything that felt wrong: a wrong result, a flicker, a delay, a window that did not go away.
  5. If something broke, please send agi.log from the folder above (Settings > About opens it).
