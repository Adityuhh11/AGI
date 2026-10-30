<script lang="ts">
  import { Service as Settings } from "../../../bindings/omni/internal/settingsui";
  import type { SearchView, View } from "../../../bindings/omni/internal/settingsui";
  import { duration } from "../format";
  import { keycaps } from "../keycaps";
  import IndexWindow from "../settings/IndexWindow.svelte";

  interface Props {
    view: View;
    search: SearchView;
    /** Whether the person set up file search in this guide. */
    files: boolean;
    refresh: () => Promise<void>;
    /** Goes back to the folders screen. */
    onfolders: () => void;
  }
  let { view, search, files, refresh, onfolders }: Props = $props();

  let error = $state("");

  const ready = $derived(search.Installed && search.Enabled && search.EngineState === "ready" && !search.NeedsUpdate);
  const percent = $derived(search.Total > 0 ? Math.min(100, Math.round((search.Done / search.Total) * 100)) : 0);
  const megabytes = (bytes: number) => (bytes / (1024 * 1024)).toFixed(0);
  const hasFolders = $derived((view.Roots ?? []).length > 0);
  // A first index of hours is worth offering the night for.
  const LONG = 2 * 3600;

  async function start() {
    error = await Settings.StartIndexing();
    await refresh();
  }
</script>

<h2>You are all set</h2>

<p class="lede">
  Press {#each keycaps(view.Hotkey) as part (part)}<kbd>{part}</kbd> {/each} in any program to open the bar. Omni lives in the notification area, by the
  clock; closing this window does not stop anything.
</p>

{#if !files}
  <p class="help">
    File search is off, so only apps, the web, the clipboard and snippets are searched. Turn it on any time in Settings > File search; it is
    the same download.
  </p>
{:else if search.Installing}
  <div class="card">
    <p class="label">Downloading file search</p>
    <div class="bar" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={percent} aria-label={search.StepName}>
      <div class="fill" style:transform="scaleX({(search.Total > 0 ? percent : 4) / 100})"></div>
    </div>
    <p class="help">
      Step {search.Step} of {search.Steps}: {search.StepName}{search.Total > 0 ? ` · ${megabytes(search.Done)} of ${megabytes(search.Total)} MB` : ""}.
      Nothing is indexed until you start it, so you can use Omni meanwhile.
    </p>
  </div>
{:else if search.Error}
  <p class="error">The download failed: {search.Error}</p>
  <div><button type="button" class="btn primary" onclick={() => Settings.TurnOn().then(refresh)}>Try again</button></div>
{:else if search.EngineState === "failed"}
  <p class="error">The search engine could not start: {search.EngineMessage}</p>
{:else if !ready}
  <div class="card">
    <p class="label">Starting the search engine</p>
    <p class="help">A few seconds.</p>
  </div>
{:else if search.HoldIndexing}
  <div class="card">
    <p class="label">Ready to index</p>
    {#if hasFolders}
      <p class="help">
        {(view.Roots ?? []).length} folder{(view.Roots ?? []).length === 1 ? "" : "s"} will be read, newest files first, and you can search while it
        works. {search.Device === "cpu" ? "On the processor it takes a while; Gentle keeps the computer free." : ""}
      </p>
      {#if search.Estimate.Complete}
        <p class="help">
          Indexing everything takes {duration(search.Estimate.Seconds)} at this speed ({duration(search.Estimate.ByPace?.fast ?? 0)} at Fast,
          {duration(search.Estimate.ByPace?.gentle ?? 0)} at Gentle). The time left shows once it is running.
        </p>
      {:else}
        <p class="help">Counting what is in your folders to say how long it will take...</p>
      {/if}
      {#if search.Estimate.Complete && search.Estimate.Seconds > LONG}
        <p class="help">That is a long time. Omni can work only at night and leave the computer alone by day.</p>
        <IndexWindow value={search.IndexBetween} onsave={async (between) => { error = await Settings.SetIndexBetween(between); await refresh(); }} />
      {/if}
      <div><button type="button" class="btn primary" onclick={start}>Start indexing</button></div>
    {:else}
      <p class="help">No folder is chosen yet.</p>
      <div><button type="button" class="btn" onclick={onfolders}>Choose folders</button></div>
    {/if}
  </div>
{:else}
  <div class="card">
    <p class="label">{search.IndexState === "uptodate" ? "Everything is indexed" : "Reading your files"}</p>
    <p class="help">{search.Paused ? "Indexing is paused." : search.IndexText}</p>
    {#if search.IndexState === "indexing" && search.IndexTotal > 0}
      <div class="bar" role="progressbar" aria-valuemin="0" aria-valuemax={search.IndexTotal} aria-valuenow={search.IndexDone} aria-label="Files indexed">
        <div class="fill" style:transform="scaleX({search.IndexDone / search.IndexTotal})"></div>
      </div>
    {/if}
    <p class="help">You can search right away: results fill in as Omni reads your files, newest first. The notification-area icon shows how it is going.</p>
  </div>
{/if}
{#if error}<p class="error">{error}</p>{/if}

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin: 4px 0 0;
    padding: 16px;
    border: 1px solid var(--line);
    border-radius: 12px;
    background: var(--tile);
  }
  .card .label {
    margin: 0;
    font: 600 13.5px/1.2 var(--sans);
  }
  .bar {
    height: 6px;
    border-radius: 3px;
    background: var(--lift);
    overflow: hidden;
  }
  .fill {
    height: 100%;
    border-radius: 3px;
    background: var(--chrome);
    transform-origin: left;
    transition: transform 0.4s var(--ease);
  }
</style>
