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
  - Clipboard history (off until you switch it on in Settings > Clipboard): copy as usual, then open
    the bar and type "clip:" and a few words. Enter pastes it into the program you came from, Ctrl+Enter
    only copies it. It keeps text only, never what a password manager marks as private, and says where.
    Settings > Clipboard can give it a hotkey of its own that opens the bar already on that list.
  - Snippets (Settings > Snippets): text you paste often, with {date}, {time} and {clipboard} filled in.
    Type "snip:" and pick one, or type its keyword. Clipboard history and Snippets start out pinned.
  - Ctrl+P pins a row (an app, a file, a quicklink; up to eight). The bar is listing your pins
    the moment it opens, and every time you open it again it starts empty, with the pins.
  - Ctrl+K opens what can be done with the selected row. Ctrl+, opens Settings, and so does typing
    "settings" in the bar: "settings file" goes straight to the File search page.

FILE SEARCH (optional, off until you turn it on)

  Finds files by what they contain, not by their names: code, PDF pages, notes, pictures by what
  they show ("the diagram with the three boxes") and videos. It is the part of AGI that needs the
  most from your computer, so it is a choice.

  Settings > File search > "Download and turn on".
  - It downloads about 1 GB once, from github.com, pypi.org, files.pythonhosted.org and
    huggingface.co, and says so before you press the button. Afterwards AGI makes no network
    calls for it. It took about two minutes on a fast connection.
  - It runs on the graphics card (NVIDIA, AMD or Intel Arc, through Vulkan) and falls back to the
    processor if there is none or its driver does not support Vulkan. On the processor a picture
    takes about 5 seconds to index instead of a fraction of one; Settings says when that is so.
  - "What to index" chooses what pictures cost you:
      PDF pages: "Pages that need it" (the default: scans, slides and pages with pictures or tables
        are also found by how they look), "Every page" (slowest) or "Words only" (fastest).
      Photos: by what they show (the default) or by their names only.
      Videos: by their names (the default) or also by what is in them (eight frames each; type
        "vid:" to search only videos, and a match says where, e.g. "at 1:23").
    On a computer without a graphics card, "Words only" and names are the quick choices. Changing
    one later redoes only that kind of file, and nothing already indexed is computed again.
  - Add the folders you want searched, one at a time (Settings > File search > Add folder).
    Nothing is read until you do, and after the download nothing is indexed until you press "Start
    indexing", so you can choose the folders and the options above first. Changing an option later
    shows what it will do and waits for "Save and re-index".
  - Indexing runs in the background. Pick how hard it works: Fast, Balanced (the default, about
    twice as long) or Gentle (about six times as long, the computer stays free). Pause and Resume
    are there too, and in the menu of the notification-area icon. Searching works throughout, on
    what is indexed so far.
  - "Turn off" stops it and keeps the files. "Delete downloaded files" removes the engine and the
    models (about 1.1 GB on disk) and keeps the index, which works again after turning it on.
  - A new version of AGI may come with a newer search engine; Settings offers an Update button.
    (0.5.0 does: press it once after updating from 0.4.0, so videos can be indexed.)
  - A folder that shows "Nothing here AGI can read" may be one Windows moved: with OneDrive's
    backup on, Documents, Desktop and Pictures live under the OneDrive folder. Settings says where.

WHERE THINGS ARE

  Settings file   %APPDATA%\AGI\config.toml
  Log             %LOCALAPPDATA%\AGI\logs\agi.log
  Search engine   %LOCALAPPDATA%\AGI\engine      (only after you turn file search on)
  Index           %LOCALAPPDATA%\AGI\memory      (what file search has indexed)
  Clipboard       %LOCALAPPDATA%\AGI\clipboard.json (only if you switched the history on)
  Quit            right-click the icon in the notification area, then Quit
  Remove it all   quit it, delete this folder, then delete the folders above.
  The program has no telemetry. Besides the downloads of file search it only touches the network
  when you open a web search or a link (your browser does that), and when you ask about updates:
  Settings > About > "Check for updates", or "Check once a day" if you turn it on (off by default).
  That request goes to api.github.com, asks for the latest version number and sends nothing else.
  "Update to X" in the same place downloads the new version only when you press it, checks it against
  the checksum GitHub published for it, replaces agi.exe where it is (the folder must be one you can
  write to) and restarts. Or download the zip yourself and replace agi.exe by hand.

NOT HERE YET

  Window management, system commands, searching Office files, what is said in a video (only what
  is seen), images in the clipboard history.

WHAT I WOULD LIKE TO KNOW

  1. Did it start without trouble? Did the hotkey work, or does something else have it?
  2. Next to what you use today: what do you reach for that is missing, slower, or in the wrong place?
  3. If you turned file search on: how long did the download take, which folders did you add, and
     did indexing get in your way (heat, fans, a slow computer, a game stuttering)? Did the speeds
     feel right? Were the results you expected first?
  4. Anything that felt wrong: a wrong result, a flicker, a delay, a window that did not go away.
  5. If something broke, please send agi.log from the folder above (Settings > About opens it).
