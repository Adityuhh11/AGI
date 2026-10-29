<script lang="ts">
  import { onMount } from "svelte";
  import { OmniAI, Service as Settings } from "../../../bindings/omni/internal/settingsui";
  import type { ClipboardView, NotesHotkeyView, OmniAIView, View } from "../../../bindings/omni/internal/settingsui";
  import HotkeyField from "./HotkeyField.svelte";

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
  let browsers = $state<string[]>([]);
  // The other hotkeys live on pages of their own services; they are shown here, beside the search bar's, and saved there.
  let clipboard = $state<ClipboardView | null>(null);
  let chat = $state<OmniAIView | null>(null);
  let notes = $state<NotesHotkeyView | null>(null);
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

  async function saveStartup() {
    error = await Settings.SetAutostart(startup);
    if (error !== "") startup = !startup;
    else flashSaved();
  }
</script>

<h2>General</h2>
<p class="lede">Hotkeys, and where web searches open. Changes apply at once.</p>

<h3>Hotkeys</h3>
<p class="help keys">Press a hotkey again while its window is in front to close it. Windows-key combinations cannot be recorded here; put them in config.toml.</p>

<div class="field">
  <span class="label" id="hotkey-label">Search bar</span>
  <HotkeyField labelId="hotkey-label" value={view.Hotkey} defaultValue={view.DefaultHotkey} onsave={saveHotkey} />
  <p class="help">Opens the search bar from any app.</p>
</div>

{#if clipboard && chat && notes}
  <div class="field">
    <span class="label" id="clip-hotkey-label">Clipboard history</span>
    <HotkeyField labelId="clip-hotkey-label" value={clipboard.Hotkey} clearable onsave={saveClipboardKey} />
    {#if clipboard.HotkeyError}<p class="error">{clipboard.HotkeyError}</p>{/if}
    <p class="help">Opens the bar already listing what you copied. Optional; Windows keeps Win+V for its own history.</p>
  </div>

  <div class="field">
    <span class="label" id="ai-hotkey-label">Omni AI</span>
    <HotkeyField labelId="ai-hotkey-label" value={chat.Hotkey} clearable onsave={saveChatKey} />
    {#if chat.HotkeyError}<p class="error">{chat.HotkeyError}</p>{/if}
    <p class="help">Opens the chat window. Optional.</p>
  </div>

  <div class="field">
    <span class="label" id="notes-hotkey-label">Notes</span>
    <HotkeyField labelId="notes-hotkey-label" value={notes.Hotkey} clearable onsave={saveNotesKey} />
    {#if notes.Error}<p class="error">{notes.Error}</p>{/if}
    <p class="help">Opens the notes window. Optional.</p>
  </div>
{/if}

<h3>Behaviour</h3>

<div class="field">
  <label class="check"><input type="checkbox" bind:checked={startup} onchange={saveStartup} /> Start when I sign in</label>
  <p class="help">The search bar is then ready as soon as Windows is. It starts quietly, with no window.</p>
</div>

<div class="field">
  <label class="check"><input type="checkbox" bind:checked={lift} onchange={saveLift} /> Lift the selected result when I use the arrow keys</label>
  <p class="help">The selected row rises a little and the others fade while you move through the list, then everything settles. Off keeps the list still.</p>
</div>

<div class="field">
  <label class="label" for="browser">Browser for web searches and links</label>
  <select id="browser" bind:value={browser} onchange={() => save()}>
    <option value="">Windows default browser</option>
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
  h3 {
    margin: 0 0 6px;
    color: var(--text-2);
    font: 600 11.5px/1.2 var(--sans);
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }
  .keys {
    margin: 0 0 18px;
  }
</style>
