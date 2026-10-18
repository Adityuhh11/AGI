<script lang="ts">
  import { onMount } from "svelte";
  import { Service as Settings } from "../../../bindings/agi/internal/settingsui";
  import type { SearchView, View } from "../../../bindings/agi/internal/settingsui";

  let { view, reload }: { view: View; reload: () => Promise<void> } = $props();

  let search = $state<SearchView | null>(null);
  let error = $state("");
  let confirming = $state("");
  let removingFiles = $state(false);
  const roots = $derived(view.Roots ?? []);

  const refresh = async () => {
    search = await Settings.Search();
  };

  // While an install runs the page asks every second for its progress; otherwise now and then, for the engine's state.
  onMount(() => {
    refresh();
    const timer = setInterval(refresh, 1000);
    return () => clearInterval(timer);
  });

  const megabytes = (bytes: number) => (bytes / (1024 * 1024)).toFixed(bytes < 10 * 1024 * 1024 ? 1 : 0);
  const percent = $derived(search && search.Total > 0 ? Math.min(100, Math.round((search.Done / search.Total) * 100)) : 0);
  const engineText = $derived.by(() => {
    if (!search) return "";
    switch (search.EngineState) {
      case "ready":
        return "Running. Search works while the index builds.";
      case "starting":
      case "warming":
        return "Starting…";
      case "restarting":
        return `Restarting: ${search.EngineMessage}`;
      case "failed":
        return `It could not start: ${search.EngineMessage}`;
      default:
        return "Off";
    }
  });

  async function run(action: () => Promise<string>) {
    error = await action();
    await refresh();
  }

  async function add() {
    error = await Settings.AddFolder();
    await reload();
  }

  async function remove(folder: string) {
    confirming = "";
    error = await Settings.RemoveFolder(folder);
    await reload();
  }

  async function removeFiles() {
    removingFiles = false;
    await run(() => Settings.RemoveFiles());
  }
</script>

<h2>File search</h2>
<p class="lede">
  Find files by what they contain: code, PDF pages, notes, and pictures by what they show. It runs on this computer and nothing leaves it.
</p>

{#if search}
  <div class="field">
    <span class="label">Status</span>

    {#if search.Installing}
      <p class="help">
        Step {search.Step} of {search.Steps}: {search.StepName}
      </p>
      <div class="bar" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={percent} aria-label={search.StepName}>
        <div class="fill" style:width="{search.Total > 0 ? percent : 4}%"></div>
      </div>
      {#if search.Total > 0}
        <p class="help mono">{megabytes(search.Done)} of {megabytes(search.Total)} MB</p>
      {:else if search.Message}
        <p class="help mono">{search.Message}</p>
      {/if}
      <div><button type="button" class="btn" onclick={() => Settings.CancelInstall().then(refresh)}>Cancel</button></div>
      <p class="help">You can close this window; the download goes on. Cancelling keeps what was downloaded.</p>
    {:else if !search.Installed}
      <p class="help">
        Turning it on downloads about {(search.DownloadMB / 1024).toFixed(1)} GB once: a search engine, its Python libraries and the models.
        Nothing is downloaded until you press the button, and nothing is sent anywhere.
      </p>
      <p class="help mono">From {(search.Hosts ?? []).join(", ")}</p>
      {#if search.Error}<p class="error">The last attempt failed: {search.Error}</p>{/if}
      <div><button type="button" class="btn primary" onclick={() => run(() => Settings.TurnOn())}>Download and turn on</button></div>
    {:else if search.NeedsUpdate}
      <p class="help">This version of AGI comes with a newer search engine than the one installed. Updating replaces it and keeps your index.</p>
      {#if search.Error}<p class="error">The last attempt failed: {search.Error}</p>{/if}
      <div><button type="button" class="btn primary" onclick={() => run(() => Settings.Update())}>Update</button></div>
    {:else if search.Enabled}
      <p class="help">{engineText}</p>
      <div><button type="button" class="btn" onclick={() => run(() => Settings.TurnOff())}>Turn off</button></div>
    {:else}
      <p class="help">Installed and off.</p>
      <div><button type="button" class="btn primary" onclick={() => run(() => Settings.TurnOn())}>Turn on</button></div>
    {/if}

    {#if search.Installed && !search.Installing}
      <div class="files">
        {#if removingFiles}
          <span class="help">Delete the downloaded engine and models? Your index stays.</span>
          <button type="button" class="btn danger" onclick={removeFiles}>Delete</button>
          <button type="button" class="btn quiet" onclick={() => (removingFiles = false)}>Keep</button>
        {:else}
          <button type="button" class="btn quiet" onclick={() => (removingFiles = true)}>Delete downloaded files…</button>
        {/if}
      </div>
    {/if}
  </div>
{/if}

<div class="field">
  <span class="label">Folders to search</span>
  {#if roots.length > 0}
    <ul class="folders">
      {#each roots as root (root)}
        <li>
          <span class="path mono" title={root}>{root}</span>
          {#if confirming === root}
            <span class="confirm">
              <button type="button" class="btn danger" onclick={() => remove(root)}>Remove</button>
              <button type="button" class="btn quiet" onclick={() => (confirming = "")}>Keep</button>
            </span>
          {:else}
            <button type="button" class="btn quiet" onclick={() => (confirming = root)}>Remove</button>
          {/if}
        </li>
      {/each}
    </ul>
  {:else}
    <p class="help">No folders yet. Nothing is searched until you add one.</p>
  {/if}
  <div><button type="button" class="btn" onclick={add}>Add folder…</button></div>
  {#if error}<p class="error">{error}</p>{/if}
</div>

<style>
  .bar {
    height: 6px;
    border-radius: 3px;
    background: var(--tile);
    overflow: hidden;
  }
  .fill {
    height: 100%;
    border-radius: 3px;
    background: var(--chrome);
    transition: width 0.4s var(--ease);
  }
  .files {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 4px;
  }
  .folders {
    display: flex;
    flex-direction: column;
    margin: 0;
    padding: 0;
    list-style: none;
    border: 1px solid var(--line);
    border-radius: 10px;
  }
  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 6px 6px 12px;
  }
  li + li {
    border-top: 1px solid var(--line);
  }
  .path {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    direction: rtl; /* a long path keeps its end, where the folder's own name is */
    text-align: left;
    color: var(--text-2);
  }
  .confirm {
    display: flex;
    gap: 4px;
  }
</style>
