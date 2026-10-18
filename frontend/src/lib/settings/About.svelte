<script lang="ts">
  import { Clipboard } from "@wailsio/runtime";
  import { Service as Settings } from "../../../bindings/agi/internal/settingsui";
  import type { View } from "../../../bindings/agi/internal/settingsui";

  let { view }: { view: View } = $props();

  let copied = $state(false);
  let error = $state("");

  async function copyDiagnostics() {
    await Clipboard.SetText(await Settings.Diagnostics());
    copied = true;
    setTimeout(() => (copied = false), 2000);
  }
</script>

<h2>About</h2>
<p class="lede">AGI {view.Version}. A launcher that keeps everything on this computer.</p>

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
