AGI {VERSION}
A launcher for Windows: press a hotkey, type, and open an app, search the web, or jump to a quicklink.
Everything runs on this computer. This build has no file search yet; it is the next thing.

STARTING IT

  1. Unzip the folder anywhere and run agi.exe.
     Windows may say "Windows protected your PC", because the program is not signed yet.
     Click "More info", then "Run anyway".
  2. There is no window of its own. The program sits in the notification area (the little arrow
     by the clock). The first time it starts it opens Settings so you can see the hotkey.
  3. Press Alt+Shift+Space to open the bar. Settings > General changes the hotkey, and says
     so if another program already owns the one you pick.

WHAT TO TRY

  - Type a few letters of an app (vsc, chr, ppt) and press Enter. An app that is already open
    is brought to the front instead of being started a second time.
  - Tab switches what the bar covers: everything, apps, files, the web. "?" or "web:" at the start
    searches the web; Alt+Enter does it from any row. It opens in the browser set in Settings.
  - Quicklinks: Settings > Quicklinks. "gh rust lifetimes" opens that search on GitHub.
  - Ctrl+T on a row tags it. "#work" lists everything tagged work.
  - Ctrl+K opens what can be done with the selected row. Ctrl+, opens Settings.

WHERE THINGS ARE

  Settings file   %APPDATA%\AGI\config.toml
  Log             %LOCALAPPDATA%\AGI\logs\agi.log
  Quit            right-click the icon in the notification area, then Quit
  Remove it all   quit it, delete this folder, then delete the two folders above.
  The program has no telemetry and does not check for updates. It only touches the network
  when you open a web search or a link, and then your browser does it.

NOT HERE YET

  File search by meaning, clipboard history, snippets, window management, system commands.

WHAT I WOULD LIKE TO KNOW

  1. Did it start without trouble? Did the hotkey work, or does something else have it?
  2. Next to what you use today: what do you reach for that is missing, slower, or in the wrong place?
  3. Anything that felt wrong: a wrong result, a flicker, a delay, a window that did not go away.
  4. If something broke, please send agi.log from the folder above.
