<script lang="ts">
  import { onMount } from "svelte";
  import { Service as Settings } from "../../../bindings/agi/internal/settingsui";
  import type { ClipboardView } from "../../../bindings/agi/internal/settingsui";
  import HotkeyField from "./HotkeyField.svelte";

  let page = $state<ClipboardView | null>(null);
  let enabled = $state(false);
  let ignore = $state("");
  let error = $state("");
  let saved = $state(false);
  let clearing = $state(false);
  let savedTimer: ReturnType<typeof setTimeout> | undefined;

  async function load() {
    page = await Settings.Clipboard();
    enabled = page.Enabled;
    ignore = (page.Ignore ?? []).join("\n");
  }
  onMount(load);

  function flashSaved() {
    saved = true;
    clearTimeout(savedTimer);
    savedTimer = setTimeout(() => (saved = false), 1800);
  }

  async function save() {
    error = await Settings.SaveClipboard(enabled, ignore.split("\n"), page?.Hotkey ?? "");
    if (error !== "") {
      enabled = page?.Enabled ?? false;
      return;
    }
    await load();
    flashSaved();
  }

  // The hotkey field shows its own refusal; the rest of the page is saved with it.
  async function saveHotkey(accelerator: string): Promise<string> {
    const refused = await Settings.SaveClipboard(enabled, ignore.split("\n"), accelerator);
    if (refused === "") {
      await load();
      flashSaved();
    }
    return refused;
  }

  async function clear() {
    clearing = false;
    error = await Settings.ClearClipboard();
    await load();
  }
</script>

<h2>Clipboard</h2>
<p class="lede">
  A history of the text you copy. Open the bar, type <span class="mono">clip:</span> and a few words, and Enter pastes it where you were.
</p>

{#if page}
  <div class="field">
    <label class="check"><input type="checkbox" bind:checked={enabled} onchange={save} /> Remember what I copy</label>
    <p class="help">
      Off until you turn it on. It keeps text only, the newest 500 copies from the last 30 days, in a file on this computer (<span class="mono">{page.File}</span>).
      Nothing leaves it. Copies a program marks as private, such as a password manager's, are never kept. Text copied from a web page or a terminal
      can still hold a secret this cannot know about: clear the history if it does.
    </p>
  </div>

  <div class="field">
    <span class="label" id="clip-hotkey-label">Hotkey</span>
    <HotkeyField labelId="clip-hotkey-label" value={page.Hotkey} clearable onsave={saveHotkey} />
    {#if page.HotkeyError}<p class="error">{page.HotkeyError}</p>{/if}
    <p class="help">Optional: opens the bar already listing what you copied. Windows keeps Win+V for its own history.</p>
  </div>

  <div class="field">
    <label class="label" for="ignore">Programs to leave out</label>
    <textarea id="ignore" rows="4" bind:value={ignore} onchange={save} spellcheck="false" placeholder="One program per line, for example MyVault.exe"></textarea>
    <p class="help">Always left out: {(page.Defaults ?? []).join(", ")}.</p>
  </div>

  <div class="field">
    <span class="label">History</span>
    <p class="help">{page.Count === 0 ? "Nothing is kept." : page.Count === 1 ? "1 copy is kept." : page.Count + " copies are kept."}</p>
    <div class="row">
      {#if clearing}
        <span class="help">Forget everything copied so far?</span>
        <button type="button" class="btn danger" onclick={clear}>Clear history</button>
        <button type="button" class="btn quiet" onclick={() => (clearing = false)}>Keep</button>
      {:else}
        <button type="button" class="btn" disabled={page.Count === 0} onclick={() => (clearing = true)}>Clear history…</button>
      {/if}
    </div>
  </div>

  {#if error}<p class="error">{error}</p>{/if}
  <p class="ok" aria-live="polite">{saved ? "Saved" : ""}</p>
{/if}

<style>
  textarea {
    box-sizing: border-box;
    width: 100%;
    max-width: 420px;
    padding: 9px 12px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--tile);
    color: var(--text);
    font: 450 13px/1.5 var(--mono);
    resize: vertical;
  }
  textarea:focus {
    outline: none;
    border-color: rgba(230, 232, 236, 0.45);
  }
  textarea::placeholder {
    color: var(--text-3);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 10px;
  }
</style>
