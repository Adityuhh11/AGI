<script lang="ts">
  import { onMount } from "svelte";
  import { Service as Settings } from "../../../bindings/agi/internal/settingsui";
  import type { SearchView, View } from "../../../bindings/agi/internal/settingsui";
  import { changes, effects, type Choices, type Kind } from "../indexingPlan";
  import { CHOICES, SPEEDS } from "./choices";
  import { choiceHints, paceHints, surveyLine } from "./hints";
  import ChoiceGroup from "./ChoiceGroup.svelte";
  import { draft } from "./draft";

  let { view, reload }: { view: View; reload: () => Promise<void> } = $props();

  let search = $state<SearchView | null>(null);
  let error = $state("");
  let confirming = $state("");
  let removingFiles = $state(false);
  const roots = $derived(view.Roots ?? []);

  const folderText = $derived(Object.fromEntries((search?.Folders ?? []).map((f) => [f.Root, f.Text])));
  const holds = $derived(Object.fromEntries((search?.Surveys ?? []).map((f) => [f.Root, surveyLine(f)])));

  // On an index that exists a click only changes the draft; "Save and re-index" is what starts the work. Before anything
  // is indexed (right after the download, or with no folders yet) there is nothing to protect, so a click applies at once.
  const staged = $derived(!!search && search.Indexed > 0 && !search.HoldIndexing);
  const saved = $derived<Choices | null>(search ? { PDFPages: search.PDFPages, Photos: search.Photos, Videos: search.Videos } : null);
  const shown = $derived($draft ?? saved);
  const plan = $derived(saved && $draft ? changes(saved, $draft) : []);

  async function choose(kind: Kind, id: string) {
    if (!saved) return;
    const next = { ...(shown ?? saved), [kind]: id };
    if (!staged) {
      await run(() => Settings.SetIndexing(next.PDFPages, next.Photos, next.Videos));
      return;
    }
    $draft = changes(saved, next).length > 0 ? next : null;
  }

  async function saveDraft() {
    if (!$draft) return;
    const next = $draft;
    $draft = null;
    await run(() => Settings.SetIndexing(next.PDFPages, next.Photos, next.Videos));
  }

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
    <span class="label">Search engine</span>

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
      <p class="help">You can close this window; the download goes on. Cancelling keeps what was downloaded. Nothing is indexed until you start it.</p>
    {:else if !search.Installed}
      <p class="help">
        Turning it on downloads about {(search.DownloadMB / 1024).toFixed(1)} GB once: a search engine, its Python libraries and the models.
        Nothing is downloaded until you press the button, and nothing is sent anywhere. Afterwards you choose what to index before anything is.
      </p>
      <p class="help">
        Saved in <span class="mono">{search.Data.Engine}</span>; the index goes in <span class="mono">{search.Data.Index}</span>.
        {#if search.Data.CanChange}
          <button type="button" class="btn quiet" onclick={() => run(() => Settings.ChooseDataFolder())}>Change folder...</button>
          {#if search.Data.Chosen}<button type="button" class="btn quiet" onclick={() => run(() => Settings.UseDefaultDataFolder())}>Use the usual place</button>{/if}
        {/if}
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

  {#if search.Enabled && search.EngineState === "ready" && search.HoldIndexing && !search.NeedsUpdate}
    <div class="start" role="status">
      <p class="label">Ready. Nothing is indexed yet.</p>
      <p class="help">
        {search.Device === "cpu" ? "It runs on the processor here, so pictures are slow." : "It runs on the graphics card."}
        Add the folders you want and choose what to index below, then start.
      </p>
      <div>
        <button type="button" class="btn primary" disabled={roots.length === 0} onclick={() => run(() => Settings.StartIndexing())}>Start indexing</button>
        {#if roots.length === 0}<span class="help">Add a folder first.</span>{/if}
      </div>
    </div>
  {/if}
{/if}

<div class="field">
  <span class="label">Folders to search</span>
  {#if roots.length > 0}
    <ul class="folders">
      {#each roots as root (root)}
        <li>
          <span class="where">
            <span class="path mono" title={root}>{root}</span>
            {#if folderText[root]}<span class="state">{folderText[root]}</span>{/if}
            {#if holds[root]}<span class="state">{holds[root]}</span>{/if}
          </span>
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

{#if search?.Installed}
  <div class="field">
    <span class="label">What to index</span>
    {#if search.Device === "cpu"}
      <p class="help warn">
        This computer has no graphics card file search can use, so it runs on the processor: each picture takes about 5 seconds to
        index. Words only and names are the quick choices here.
      </p>
    {/if}
    {#each CHOICES as choice (choice.kind)}
      <span class="sublabel" id="{choice.kind}-label">{choice.label}</span>
      <ChoiceGroup labelId="{choice.kind}-label" options={choice.options} value={shown?.[choice.kind]} onpick={(id) => choose(choice.kind, id)} hints={choiceHints(choice.kind, choice.options.map((o) => o.id), search.Estimate)} />
    {/each}
    {#if plan.length > 0}
      <div class="draft" role="status">
        <p class="label">Not saved yet</p>
        <ul>
          {#each effects(plan, search.Device) as line (line)}<li>{line}</li>{/each}
        </ul>
        <div>
          <button type="button" class="btn primary" onclick={saveDraft}>Save and re-index</button>
          <button type="button" class="btn quiet" onclick={() => ($draft = null)}>Discard</button>
        </div>
      </div>
    {:else}
      <p class="help">
        {staged
          ? "A change is saved with a button, and then only that kind of file is looked at again; nothing already indexed is computed again."
          : "Choose before you start: changing these later re-indexes the files they concern."}
      </p>
    {/if}
  </div>

  <div class="field">
    <span class="label">Indexing</span>
    {#if search.Enabled && search.EngineState === "ready" && !search.NeedsUpdate && !search.HoldIndexing}
      <p class="help">{search.Paused ? "Indexing is paused. Search still works on what is indexed." : search.IndexText}</p>
      {#if search.IndexState === "indexing" && !search.Paused}
        <p class="help">You can search already: results fill in as files are read, newest first.</p>
      {/if}
      {#if search.IndexState === "indexing" && search.IndexTotal > 0}
        <div class="bar" role="progressbar" aria-valuemin="0" aria-valuemax={search.IndexTotal} aria-valuenow={search.IndexDone} aria-label="Files indexed in this folder">
          <div class="fill" style:width="{Math.round((search.IndexDone / search.IndexTotal) * 100)}%"></div>
        </div>
      {/if}
      <div>
        <button type="button" class="btn" onclick={() => (search?.Paused ? Settings.ResumeIndexing() : Settings.PauseIndexing()).then(refresh)}>
          {search.Paused ? "Resume indexing" : "Pause indexing"}
        </button>
      </div>
    {:else}
      <p class="help">Nothing is being indexed.</p>
    {/if}
    <span class="sublabel" id="speed-label">How hard it works</span>
    <ChoiceGroup labelId="speed-label" options={SPEEDS} value={search.Pace} onpick={(id) => run(() => Settings.SetPace(id))} hints={paceHints(search.Estimate)} />
    <p class="help">Measured on a laptop with a 6 GB card. The index is built once; after that only what changes is read.</p>
  </div>
{/if}

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
  .start,
  .draft {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin: 0 0 22px;
    padding: 14px;
    border: 1px solid rgba(230, 232, 236, 0.45);
    border-radius: 10px;
    background: var(--lift);
  }
  .draft {
    margin: 6px 0 0;
  }
  .start .label,
  .draft .label {
    margin: 0;
    color: var(--text);
    font: 560 13px/1.2 var(--sans);
  }
  .draft ul {
    margin: 0;
    padding-left: 18px;
    color: var(--text-2);
    font-size: 12px;
    line-height: 1.5;
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
  .where {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .state {
    color: var(--text-3);
    font-size: 11.5px;
  }
  .sublabel {
    margin-top: 6px;
    color: var(--text-2);
    font-size: 12px;
  }
  .warn {
    color: var(--danger);
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
