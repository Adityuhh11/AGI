<script lang="ts">
  import { Clipboard } from "@wailsio/runtime";
  import { Service as Settings } from "../../../bindings/agi/internal/settingsui";
  import type { View } from "../../../bindings/agi/internal/settingsui";

  let { view }: { view: View } = $props();

  let copied = $state(false);
  let error = $state("");
  // svelte-ignore state_referenced_locally
  let found = $state(view.UpdateFound);
  // svelte-ignore state_referenced_locally
  let daily = $state(view.CheckUpdates);
  let checking = $state(false);
  let outcome = $state("");

  async function check() {
    checking = true;
    outcome = "";
    const r = await Settings.CheckUpdate();
    checking = false;
    if (r.Error) {
      outcome = "Could not check: " + r.Error;
      return;
    }
    found = r.Available ? r.Latest : "";
    outcome = r.Available ? "" : "No release is newer than " + view.Version + ".";
  }

  async function saveDaily() {
    error = await Settings.SetCheckUpdates(daily);
    if (error !== "") daily = !daily;
    else if (daily) check();
  }

  async function copyDiagnostics() {
    await Clipboard.SetText(await Settings.Diagnostics());
    copied = true;
    setTimeout(() => (copied = false), 2000);
  }
</script>

<h2>About</h2>
<p class="lede">AGI {view.Version}. A launcher that keeps everything on this computer.</p>

<div class="field">
  <span class="label">Updates</span>
  {#if found}
    <p class="help">Version {found} is available.</p>
    <div><button type="button" class="btn primary" onclick={async () => (error = await Settings.OpenUpdate())}>Open the release page</button></div>
  {:else if outcome}
    <p class="help">{outcome}</p>
  {/if}
  <div><button type="button" class="btn" disabled={checking} onclick={check}>{checking ? "Checking..." : "Check for updates"}</button></div>
  <label class="check"><input type="checkbox" bind:checked={daily} onchange={saveDaily} /> Check once a day</label>
  <p class="help">Asks api.github.com for the latest version number and sends nothing else. Off unless you turn it on. Installing is up to you: the release page has the download.</p>
</div>

<div class="field">
  <span class="label">Settings file</span>
  <p class="help mono">{view.ConfigPath}</p>
  <div><button type="button" class="btn" onclick={async () => (error = await Settings.OpenFolder("config"))}>Open folder</button></div>
</div>

<div class="field">
  <span class="label">Log</span>
  <p class="help">If something does not work, send the file agi.log from this folder.</p>
  <p class="help mono">{view.LogDir}</p>
  <div><button type="button" class="btn" onclick={async () => (error = await Settings.OpenFolder("logs"))}>Open folder</button></div>
</div>

<div class="field">
  <span class="label">Diagnostics</span>
  <p class="help">Version, Windows, and whether file search is running. It names no files and no folders.</p>
  <div><button type="button" class="btn" onclick={copyDiagnostics}>{copied ? "Copied" : "Copy diagnostics"}</button></div>
</div>
{#if error}<p class="error">{error}</p>{/if}
