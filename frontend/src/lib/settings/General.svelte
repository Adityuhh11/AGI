<script lang="ts">
  import { onMount } from "svelte";
  import { Service as Settings } from "../../../bindings/agi/internal/settingsui";
  import type { View } from "../../../bindings/agi/internal/settingsui";
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

  onMount(async () => {
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
<p class="lede">The hotkey and where web searches open. Changes apply at once.</p>

<div class="field">
  <span class="label" id="hotkey-label">Hotkey</span>
  <HotkeyField labelId="hotkey-label" value={view.Hotkey} defaultValue={view.DefaultHotkey} onsave={saveHotkey} />
  <p class="help">Opens the search bar from any app. Windows-key combinations cannot be recorded here; put them in config.toml.</p>
</div>

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
