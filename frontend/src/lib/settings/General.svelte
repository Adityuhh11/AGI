<script lang="ts">
  import { isMac, words } from "../platform";
  import { onMount } from "svelte";
  import { OmniAI, Service as Settings } from "../../../bindings/omni/internal/settingsui";
  import type { ClipboardView, NotesHotkeyView, OmniAIView, View } from "../../../bindings/omni/internal/settingsui";
  import HotkeyField from "./HotkeyField.svelte";
  import { applyTheme, THEMES } from "../theme";

  let { view }: { view: View } = $props();

  // The form starts from the settings as they were when the page opened and then owns its own values.
  // svelte-ignore state_referenced_locally
  let hotkey = $state(view.Hotkey);
  // svelte-ignore state_referenced_locally
  let browser = $state(view.Browser);
  // svelte-ignore state_referenced_locally
  let searchURL = $state(view.SearchURL);
  // svelte-ignore state_referenced_locally
  let startup = $state(view.Autostart);
  // svelte-ignore state_referenced_locally
  let lift = $state(view.SelectionEffect);
  // svelte-ignore state_referenced_locally
  let theme = $state(view.Theme);
  let browsers = $state<string[]>([]);
  // The other hotkeys live on pages of their own services; they are shown here, beside the search bar's, and saved there.
  let clipboard = $state<ClipboardView | null>(null);
  let chat = $state<OmniAIView | null>(null);
  let notes = $state<NotesHotkeyView | null>(null);
  let keysOpen = $state(false);
  const keysSet = $derived([view.Hotkey, clipboard?.Hotkey, chat?.Hotkey, notes?.Hotkey].filter(Boolean).length);
  let error = $state("");
  let saved = $state(false);
  let savedTimer: ReturnType<typeof setTimeout> | undefined;

  const NAMES: Record<string, string> = {
    chrome: "Chrome",
    msedge: "Edge",
    firefox: "Firefox",
    zen: "Zen",
    brave: "Brave",
    vivaldi: "Vivaldi",
    opera: "Opera",
    librewolf: "LibreWolf",
    waterfox: "Waterfox",
    floorp: "Floorp",
  };
  // A browser set by path in config.toml is not in the installed list; keep it selectable.
  const options = $derived(browser === "" || browsers.includes(browser) ? browsers : [browser, ...browsers]);
  const QUERY = "{q}";

  async function loadHotkeys() {
    [clipboard, chat, notes] = await Promise.all([Settings.Clipboard(), OmniAI.View(), Settings.NotesHotkey()]);
  }

  onMount(async () => {
    loadHotkeys();
    browsers = (await Settings.Browsers()) ?? [];
  });

  function flashSaved() {
    saved = true;
    clearTimeout(savedTimer);
    savedTimer = setTimeout(() => (saved = false), 1800);
  }

  async function save() {
    error = await Settings.SaveGeneral(hotkey, browser, searchURL);
    if (error === "") flashSaved();
  }

  // The hotkey field shows its own refusal ("that hotkey is taken"); the page only learns what was saved.
  async function saveHotkey(accelerator: string): Promise<string> {
    const refused = await Settings.SaveGeneral(accelerator, browser, searchURL);
    if (refused === "") {
      hotkey = accelerator;
      flashSaved();
    }
    return refused;
  }

  // Each field shows its own refusal; the page reloads the hotkeys so an error from startup goes away once the key works.
  async function afterKey(refused: string): Promise<string> {
    if (refused === "") {
      await loadHotkeys();
      flashSaved();
    }
    return refused;
  }
  const saveClipboardKey = async (accelerator: string) => {
    const now = await Settings.Clipboard();
    return afterKey(await Settings.SaveClipboard(now.Enabled, now.Ignore ?? [], accelerator));
  };
  const saveChatKey = async (accelerator: string) => afterKey(await OmniAI.SetHotkey(accelerator));
  const saveNotesKey = async (accelerator: string) => afterKey(await Settings.SetNotesHotkey(accelerator));

  async function saveLift() {
    error = await Settings.SetSelectionEffect(lift);
    if (error !== "") lift = !lift;
    else flashSaved();
  }

  async function saveTheme(id: string) {
    const before = theme;
    theme = id;
    applyTheme(id);
    error = await Settings.SetTheme(id);
    if (error !== "") {
      theme = before;
      applyTheme(before);
    } else flashSaved();
  }

  async function saveStartup() {
    error = await Settings.SetAutostart(startup);
    if (error !== "") startup = !startup;
    else flashSaved();
  }
</script>

<h2>General</h2>
<p class="lede">Hotkeys, and where web searches open. Changes apply at once.</p>

<!-- Four hotkeys would fill the page: they are one list that opens from its heading. -->
<div class="keys">
  <button type="button" class="head" aria-expanded={keysOpen} aria-controls="hotkey-list" onclick={() => (keysOpen = !keysOpen)}>
    <span class="chevron" class:open={keysOpen} aria-hidden="true"></span>Hotkeys
    <span class="count">{keysSet} set</span>
  </button>
  {#if keysOpen}
    <ul id="hotkey-list">
      <li title="Opens the search bar from any app. A hotkey pressed again while its window is in front closes it.">
        <span class="label" id="hotkey-label">Search bar</span>
        <HotkeyField labelId="hotkey-label" value={view.Hotkey} defaultValue={view.DefaultHotkey} onsave={saveHotkey} />
      </li>
      {#if clipboard && chat && notes}
        <li title={isMac ? "Opens the bar already listing what you copied." : "Opens the bar already listing what you copied. Windows keeps Win+V for its own history."}>
          <span class="label" id="clip-hotkey-label">Clipboard history</span>
          <HotkeyField labelId="clip-hotkey-label" value={clipboard.Hotkey} clearable onsave={saveClipboardKey} />
        </li>
        {#if clipboard.HotkeyError}<li class="error">{clipboard.HotkeyError}</li>{/if}
        <li title="Opens the chat window.">
          <span class="label" id="ai-hotkey-label">Omni AI</span>
          <HotkeyField labelId="ai-hotkey-label" value={chat.Hotkey} clearable onsave={saveChatKey} />
        </li>
        {#if chat.HotkeyError}<li class="error">{chat.HotkeyError}</li>{/if}
        <li title="Opens the notes window.">
          <span class="label" id="notes-hotkey-label">Notes</span>
          <HotkeyField labelId="notes-hotkey-label" value={notes.Hotkey} clearable onsave={saveNotesKey} />
        </li>
        {#if notes.Error}<li class="error">{notes.Error}</li>{/if}
      {/if}
    </ul>
    {#if !isMac}<p class="help">Windows-key combinations cannot be recorded here; put them in config.toml.</p>{/if}
  {/if}
</div>


<div class="field">
  <span class="label" id="theme-label">Theme</span>
  <div class="themes" role="radiogroup" aria-labelledby="theme-label">
    {#each THEMES as t (t.id)}
      <button type="button" role="radio" aria-checked={theme === t.id} class:on={theme === t.id} onclick={() => saveTheme(t.id)}>
        <span class="swatch" style:background={t.plate} style:color={t.text}>Aa</span>
        {t.label}
      </button>
    {/each}
  </div>
  <p class="help">The colours of the search bar and of every window. They change at once.</p>
</div>

<div class="field">
  <label class="check"><input type="checkbox" bind:checked={startup} onchange={saveStartup} /> Start when I sign in</label>
  <p class="help">The search bar is then ready as soon as {isMac ? "your Mac" : "Windows"} is. It starts quietly, with no window.</p>
</div>

<div class="field">
  <label class="check"><input type="checkbox" bind:checked={lift} onchange={saveLift} /> Lift the selected result when I use the arrow keys</label>
  <p class="help">The selected row rises a little and the others fade while you move through the list, then everything settles. Off keeps the list still.</p>
</div>

<div class="field">
  <label class="label" for="browser">Browser for web searches and links</label>
  <select id="browser" bind:value={browser} onchange={() => save()}>
    <option value="">{words().defaultBrowser}</option>
    {#each options as name (name)}
      <option value={name}>{NAMES[name] ?? name}</option>
    {/each}
  </select>
</div>

<div class="field">
  <label class="label" for="search-url">Web search address</label>
  <input
    id="search-url"
    type="text"
    class="mono"
    placeholder={"https://duckduckgo.com/?q=" + QUERY}
    bind:value={searchURL}
    onchange={() => save()}
    spellcheck="false"
  />
  <p class="help">Put {QUERY} where the words go. Empty uses the search engine set inside Firefox-based browsers, and DuckDuckGo otherwise.</p>
</div>

{#if error}<p class="error">{error}</p>{/if}
<p class="ok" aria-live="polite">{saved ? "Saved" : ""}</p>

<style>
  .keys {
    margin-bottom: 22px;
    border: 1px solid var(--line);
    border-radius: 12px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 11px 14px;
    border: 0;
    background: none;
    color: var(--text);
    font: 560 13px/1.2 var(--sans);
    text-align: left;
    cursor: pointer;
  }
  .head:focus-visible {
    outline: 1px solid var(--chrome);
    outline-offset: -2px;
    border-radius: 12px;
  }
  .chevron {
    flex: none;
    width: 6px;
    height: 6px;
    margin: 0 4px 0 2px;
    border-right: 1.5px solid var(--text-3);
    border-bottom: 1.5px solid var(--text-3);
    transform: rotate(-45deg);
    transition: transform 0.12s;
  }
  .chevron.open {
    transform: rotate(45deg);
  }
  .count {
    margin-left: auto;
    color: var(--text-3);
    font-weight: 450;
    font-size: 12px;
  }
  ul {
    margin: 0;
    padding: 0 14px 4px;
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 0;
    border-top: 1px solid var(--line);
  }
  li.error {
    padding-top: 0;
    border-top: 0;
    color: var(--danger);
    font-size: 12px;
  }
  .label {
    font: 560 13px/1.2 var(--sans);
  }
  .keys .help {
    margin: 0;
    padding: 0 14px 12px;
  }
  .themes {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 8px;
  }
  .themes button {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 5px 12px 5px 5px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--tile);
    color: var(--text-2);
    font: 500 13px/1 var(--sans);
    cursor: pointer;
  }
  .themes button:hover {
    background: var(--lift);
  }
  .themes button.on {
    border-color: color-mix(in srgb, var(--chrome) 45%, transparent);
    color: var(--text);
  }
  .swatch {
    display: grid;
    place-items: center;
    width: 30px;
    height: 26px;
    border: 1px solid color-mix(in srgb, var(--ink) 25%, transparent);
    border-radius: 7px;
    font: 620 12px/1 var(--sans);
  }
  @media (prefers-reduced-motion: reduce) {
    .chevron {
      transition: none;
    }
  }
</style>