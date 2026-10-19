<script lang="ts">
  import { onMount } from "svelte";
  import { Service as Settings } from "../../../bindings/agi/internal/settingsui";
  import type { View } from "../../../bindings/agi/internal/settingsui";
  import { capture } from "../accelerator";

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
  let browsers = $state<string[]>([]);
  let capturing = $state(false);
  let hint = $state("");
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

  async function save(nextHotkey = hotkey) {
    error = await Settings.SaveGeneral(nextHotkey, browser, searchURL);
    if (error !== "") return;
    hotkey = nextHotkey;
    flashSaved();
  }

  async function saveStartup() {
    error = await Settings.SetAutostart(startup);
    if (error !== "") startup = !startup;
    else flashSaved();
  }

  function startCapture() {
    capturing = true;
    hint = "";
  }

  function onKeydown(e: KeyboardEvent) {
    if (!capturing) {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        startCapture();
      }
      return;
    }
    e.preventDefault();
    e.stopPropagation();
    if (e.code === "Escape" && !e.ctrlKey && !e.altKey && !e.shiftKey && !e.metaKey) {
      capturing = false;
      hint = "";
      return;
    }
    const got = capture(e);
    if (got === null) return; // only modifiers so far
    if ("hint" in got) {
      hint = got.hint;
      return;
    }
    capturing = false;
    hint = "";
    save(got.accelerator);
  }
</script>

<h2>General</h2>
<p class="lede">The hotkey and where web searches open. Changes apply at once.</p>

<div class="field">
  <span class="label" id="hotkey-label">Hotkey</span>
  <div class="keyrow">
    <button type="button" class="keys" aria-labelledby="hotkey-label" onclick={startCapture} onkeydown={onKeydown} onblur={() => (capturing = false)}>
      {#if capturing}
        <span class="prompt">Press the new combination. Esc cancels.</span>
      {:else}
        {#each hotkey.split("+") as part (part)}<kbd>{part}</kbd>{/each}
        <span class="change">Change</span>
      {/if}
    </button>
    {#if hotkey !== view.DefaultHotkey}
      <button type="button" class="btn quiet" onclick={() => save(view.DefaultHotkey)}>Reset to {view.DefaultHotkey}</button>
    {/if}
  </div>
  {#if hint}<p class="help">{hint}</p>{/if}
  {#if error && error.includes("hotkey")}<p class="error">{error}</p>{/if}
  <p class="help">Opens the search bar from any app. Windows-key combinations cannot be recorded here; put them in config.toml.</p>
</div>

<div class="field">
  <label class="check"><input type="checkbox" bind:checked={startup} onchange={saveStartup} /> Start when I sign in</label>
  <p class="help">The search bar is then ready as soon as Windows is. It starts quietly, with no window.</p>
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

{#if error && !error.includes("hotkey")}<p class="error">{error}</p>{/if}
<p class="ok" aria-live="polite">{saved ? "Saved" : ""}</p>

<style>
  .check {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--text);
    font: 560 13px/1.2 var(--sans);
    cursor: pointer;
  }
  .check input {
    accent-color: var(--text-2);
  }
  .keyrow {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .keys {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-height: 40px;
    padding: 6px 12px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--tile);
    color: var(--text-2);
    font: 450 13px/1 var(--sans);
    cursor: pointer;
  }
  .keys:hover {
    background: var(--lift);
  }
  .change {
    margin-left: 8px;
    color: var(--text-3);
    font-size: 12px;
  }
  .prompt {
    color: var(--text);
  }
</style>
