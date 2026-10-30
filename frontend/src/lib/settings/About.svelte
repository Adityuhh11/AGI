<script lang="ts">
  import { isMac } from "../platform";
  import { onDestroy, onMount } from "svelte";
  import { Clipboard } from "@wailsio/runtime";
  import { Service as Settings } from "../../../bindings/omni/internal/settingsui";
  import type { UpdateProgress, View } from "../../../bindings/omni/internal/settingsui";

  let { view }: { view: View } = $props();

  let copied = $state(false);
  let error = $state("");
  // svelte-ignore state_referenced_locally
  let found = $state(view.UpdateFound);
  // svelte-ignore state_referenced_locally
  let installable = $state(view.UpdateInstallable);
  // svelte-ignore state_referenced_locally
  let daily = $state(view.CheckUpdates);
  let checking = $state(false);
  let outcome = $state("");
  let progress = $state<UpdateProgress | null>(null);
  let pollTimer: ReturnType<typeof setTimeout> | undefined;
  const busy = $derived(progress?.Stage === "downloading" || progress?.Stage === "installing" || progress?.Stage === "done");

  // While an update installs the page asks how it is going; it stops asking when it is done or has failed.
  async function poll() {
    clearTimeout(pollTimer);
    progress = await Settings.UpdateProgress();
    if (progress.Stage === "downloading" || progress.Stage === "installing") pollTimer = setTimeout(poll, 400);
  }
  onMount(poll);
  onDestroy(() => clearTimeout(pollTimer));

  async function startUpdate() {
    error = await Settings.StartUpdate();
    if (error === "") poll();
  }

  const megabytes = (bytes: number) => (bytes / 1048576).toFixed(1);
  const stageText = $derived.by(() => {
    if (!progress) return "";
    if (progress.Stage === "downloading") {
      return progress.Total > 0 ? `Downloading ${megabytes(progress.Done)} of ${megabytes(progress.Total)} MB` : "Downloading";
    }
    if (progress.Stage === "installing") return "Checking and installing";
    return progress.Error === "" ? "Installed. The program restarts now." : "";
  });

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
    installable = r.Installable;
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
<p class="lede">Omni {view.Version}. A launcher that keeps everything on this computer.</p>

<div class="field">
  <span class="label">Updates</span>
  {#if found}
    <p class="help">Version {found} is available.</p>
    {#if busy}
      <p class="help" aria-live="polite">{stageText}</p>
      {#if progress?.Stage === "downloading" && progress.Total > 0}<progress max={progress.Total} value={progress.Done}></progress>{/if}
      {#if progress?.Stage === "downloading"}<div><button type="button" class="btn quiet" onclick={() => Settings.CancelUpdate()}>Cancel</button></div>{/if}
      {#if progress?.Stage === "done" && progress.Error}<p class="error">{progress.Error}</p>{/if}
    {:else}
      <div class="row">
        {#if installable}<button type="button" class="btn primary" onclick={startUpdate}>Update to {found}</button>{/if}
        <button type="button" class="btn {installable ? 'quiet' : 'primary'}" onclick={async () => (error = await Settings.OpenUpdate())}>Open the release page</button>
      </div>
      {#if !installable}
        <p class="help">
          {#if isMac}
            Omni cannot replace itself on a Mac yet. Run the install line from the release page in Terminal again; it keeps your settings and your index.
          {:else}
            This release has no download that can be checked, so it cannot be installed from here.
          {/if}
        </p>
      {/if}
      {#if progress?.Stage === "failed"}<p class="error">{progress.Error} You can try again, or download it from the release page.</p>{/if}
    {/if}
  {:else if outcome}
    <p class="help">{outcome}</p>
  {/if}
  <div><button type="button" class="btn" disabled={checking} onclick={check}>{checking ? "Checking..." : "Check for updates"}</button></div>
  <label class="check"><input type="checkbox" bind:checked={daily} onchange={saveDaily} /> Check once a day</label>
  <p class="help">Asks api.github.com for the latest version number and sends nothing else. Off unless you turn it on. Nothing is downloaded until you press the update button.</p>
</div>

<div class="field">
  <span class="label">Setup guide</span>
  <p class="help">The short tour of the hotkey and the bar, and the file search set-up, again.</p>
  <div><button type="button" class="btn" onclick={() => Settings.OpenSetup()}>Open the guide</button></div>
</div>

<div class="field">
  <span class="label">Settings file</span>
  <p class="help mono">{view.ConfigPath}</p>
  <div><button type="button" class="btn" onclick={async () => (error = await Settings.OpenFolder("config"))}>Open folder</button></div>
</div>

<div class="field">
  <span class="label">Log</span>
  <p class="help">If something does not work, send the file omni.log from this folder.</p>
  <p class="help mono">{view.LogDir}</p>
  <div><button type="button" class="btn" onclick={async () => (error = await Settings.OpenFolder("logs"))}>Open folder</button></div>
</div>

<div class="field">
  <span class="label">Diagnostics</span>
  <p class="help">Version, {isMac ? "macOS" : "Windows"}, and whether file search is running. It names no files and no folders.</p>
  <div><button type="button" class="btn" onclick={copyDiagnostics}>{copied ? "Copied" : "Copy diagnostics"}</button></div>
</div>
{#if error}<p class="error">{error}</p>{/if}

<style>
  .row {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  progress {
    width: 100%;
    max-width: 320px;
    height: 6px;
    accent-color: var(--chrome);
  }
</style>
