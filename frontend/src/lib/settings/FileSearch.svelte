<script lang="ts">
  import { onMount } from "svelte";
  import { Service as Settings } from "../../../bindings/omni/internal/settingsui";
  import type { SearchView, View } from "../../../bindings/omni/internal/settingsui";
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
  let removingAll = $state(false);
  let optionsOpen = $state(false);
  const roots = $derived(view.Roots ?? []);

  const folderText = $derived(Object.fromEntries((search?.Folders ?? []).map((f) => [f.Root, f.Text])));
  const refused = $derived(new Set((search?.Folders ?? []).filter((f) => f.Blocked).map((f) => f.Root)));
  const holds = $derived(Object.fromEntries((search?.Surveys ?? []).map((f) => [f.Root, surveyLine(f)])));

  // On an index that exists a click only changes the draft; "Save and re-index" is what starts the work. Before anything
  // is indexed (right after the download, or with no folders yet) there is nothing to protect, so a click applies at once.
  const staged = $derived(!!search && search.Indexed > 0 && !search.HoldIndexing);
  const saved = $derived<Choices | null>(search ? { PDFPages: search.PDFPages, Photos: search.Photos, Videos: search.Videos, Audio: search.Audio } : null);
  const shown = $derived($draft ?? saved);
  const plan = $derived(saved && $draft ? changes(saved, $draft) : []);

  async function choose(kind: Kind, id: string) {
    if (!saved) return;
    const next = { ...(shown ?? saved), [kind]: id };
    if (!staged) {
      await run(() => Settings.SetIndexing(next.PDFPages, next.Photos, next.Videos, next.Audio));
      return;
    }
    $draft = changes(saved, next).length > 0 ? next : null;
  }

  async function saveDraft() {
    if (!$draft) return;
    const next = $draft;
    $draft = null;
    await run(() => Settings.SetIndexing(next.PDFPages, next.Photos, next.Videos, next.Audio));
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

  async function removeEverything() {
    removingAll = false;
    $draft = null;
    await run(() => Settings.RemoveEverything());
    await reload();
  }

  // The options are folded away until they matter: before the first indexing, or with a change waiting to be saved.
  $effect(() => {
    if (search?.HoldIndexing || plan.length > 0) optionsOpen = true;
  });

  const running = $derived(!!search && search.Enabled && search.Installed && !search.NeedsUpdate && search.EngineState === "ready");
  const failed = $derived(!!search && (search.EngineState === "failed" || !!search.Error));
  const headline = $derived.by(() => {
    if (!search) return "";
    if (search.Installing) return "Downloading";
    if (!search.Installed) return "Not installed";
    if (search.NeedsUpdate) return "Needs an update";
    return search.Enabled ? "On" : "Off";
  });

  async function removeFiles() {
    removingFiles = false;
    await run(() => Settings.RemoveFiles());
  }
</script>
<h2>File search</h2>
<p class="lede">Find files by what they contain. It runs on this computer and nothing leaves it.</p>

{#if search}
  <section class="block">
    <header class="top">
      <span class="dot" class:on={running} class:bad={failed} aria-hidden="true"></span>
      <div class="who">
        <h3>{headline}</h3>
        {#if search.Installed && !search.Installing && !search.NeedsUpdate}<p class="help">{search.Enabled ? engineText : "Installed and off."}</p>{/if}
      </div>
      {#if search.Installing}
        <button type="button" class="btn" onclick={() => Settings.CancelInstall().then(refresh)}>Cancel</button>
      {:else if !search.Installed}
        <button type="button" class="btn primary" onclick={() => run(() => Settings.TurnOn())}>Download and turn on</button>
      {:else if search.NeedsUpdate}
        <button type="button" class="btn primary" onclick={() => run(() => Settings.Update())}>Update</button>
      {:else if search.Enabled}
        <button type="button" class="btn" onclick={() => run(() => Settings.TurnOff())}>Turn off</button>
      {:else}
        <button type="button" class="btn primary" onclick={() => run(() => Settings.TurnOn())}>Turn on</button>
      {/if}
    </header>

    {#if search.Installing}
      <div class="bar" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={percent} aria-label={search.StepName}>
        <div class="fill" style:transform="scaleX({(search.Total > 0 ? percent : 4) / 100})"></div>
      </div>
      <p class="help">
        Step {search.Step} of {search.Steps}: {search.StepName}{#if search.Total > 0}, {megabytes(search.Done)} of {megabytes(search.Total)} MB{/if}.
        You can close this window; the download goes on.
      </p>
    {:else if !search.Installed}
      <p class="help">
        A one-time download of about {(search.DownloadMB / 1024).toFixed(1)} GB from {(search.Hosts ?? []).join(", ")}. Nothing is downloaded until you press the
        button, and nothing is indexed until you say so.
      </p>
      <p class="help">
        Saved in <span class="mono">{search.Data.Engine}</span>.
        {#if search.Data.CanChange}
          <button type="button" class="btn quiet" onclick={() => run(() => Settings.ChooseDataFolder())}>Change…</button>
          {#if search.Data.Chosen}<button type="button" class="btn quiet" onclick={() => run(() => Settings.UseDefaultDataFolder())}>Use the usual place</button>{/if}
        {/if}
      </p>
    {:else if search.NeedsUpdate}
      <p class="help">This version of Omni comes with a newer search engine. Updating keeps your index.</p>
    {:else if search.Enabled && search.EngineState === "ready" && !search.HoldIndexing}
      <p class="help">{search.Paused ? "Indexing is paused." : search.IndexText}</p>
      {#if search.IndexState === "indexing" && search.IndexTotal > 0}
        <div class="bar" role="progressbar" aria-valuemin="0" aria-valuemax={search.IndexTotal} aria-valuenow={search.IndexDone} aria-label="Files indexed in this folder">
          <div class="fill" style:transform="scaleX({search.IndexDone / search.IndexTotal})"></div>
        </div>
      {/if}
      {#if search.IndexState === "indexing"}
        <div>
          <button type="button" class="btn" onclick={() => (search?.Paused ? Settings.ResumeIndexing() : Settings.PauseIndexing()).then(refresh)}>
            {search.Paused ? "Resume" : "Pause"}
          </button>
        </div>
      {/if}
    {/if}
    {#if search.Error}<p class="error">The last attempt failed: {search.Error}</p>{/if}
  </section>

  {#if search.Enabled && search.EngineState === "ready" && search.HoldIndexing && !search.NeedsUpdate}
    <section class="block callout" role="status">
      <h3>Ready. Nothing is indexed yet.</h3>
      <p class="help">
        {search.Device === "cpu" ? "It runs on the processor here, so pictures are slow." : "It runs on the graphics card."}
        Add your folders, check the indexing options below, then start.
      </p>
      <div>
        <button type="button" class="btn primary" disabled={roots.length === 0} onclick={() => run(() => Settings.StartIndexing())}>Start indexing</button>
        {#if roots.length === 0}<span class="help">Add a folder first.</span>{/if}
      </div>
    </section>
  {/if}
{/if}

<section class="block">
  <h3>Folders</h3>
  {#if roots.length > 0}
    <ul class="folders">
      {#each roots as root (root)}
        <li>
          <span class="where">
            <span class="path mono" title={root}>{root}</span>
            {#if folderText[root]}<span class="state">{folderText[root]}</span>{/if}
            {#if refused.has(root)}
              <button type="button" class="btn quiet" onclick={() => Settings.OpenFileAccessSettings()}>Open System Settings</button>
            {/if}
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
</section>

{#if search?.Installed}
  <section class="block">
    <h3>In the launcher</h3>
    <label class="check">
      <input type="checkbox" checked={search.InLauncher} onchange={(e) => run(() => Settings.SetInLauncher(e.currentTarget.checked))} />
      Include files in the default search
    </label>
    <p class="help">Off, the default search only finds apps, the web, clipboard and snippets. Press Tab to switch to Files and search your files.</p>
    <label class="check">
      <input type="checkbox" checked={search.GroupResults} onchange={(e) => run(() => Settings.SetGroupResults(e.currentTarget.checked))} />
      Group files by type
    </label>
    <p class="help">Shows the best match first, then Code, Documents, PDFs, Images, Videos and Audio under their own headings. Off, files are one list in the order found.</p>
  </section>

  <section class="block">
    <details bind:open={optionsOpen}>
      <summary>Indexing options</summary>
      <div class="options">
        {#if search.Device === "cpu"}
          <p class="help warn">
            No usable graphics card, so each picture takes about 5 seconds to index. Words only and names are the quick choices.
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
        {/if}
        <span class="sublabel" id="speed-label">How hard it works</span>
        <ChoiceGroup labelId="speed-label" options={SPEEDS} value={search.Pace} onpick={(id) => run(() => Settings.SetPace(id))} hints={paceHints(search.Estimate)} />
        <label class="check">
          <input type="checkbox" checked={search.PauseOnBattery} onchange={(e) => run(() => Settings.SetPauseOnBattery(e.currentTarget.checked))} />
          Wait for power
        </label>
        <p class="help">Indexing stops while the computer runs on its battery and goes on when it is plugged in. Searching is not affected.</p>
      </div>
    </details>
  </section>
{/if}

{#if search && !search.Installing}
  <section class="block remove">
    {#if removingAll}
      <p class="help">
        This deletes the engine, the models and the whole index, forgets the folders and the options, and turns file search off. To use it again you
        download it again.
      </p>
      <div class="row">
        <button type="button" class="btn danger" onclick={removeEverything}>Delete everything</button>
        <button type="button" class="btn quiet" onclick={() => (removingAll = false)}>Keep</button>
      </div>
    {:else if removingFiles}
      <p class="help">Delete the downloaded engine and models? Your index stays, and works again once the engine is downloaded.</p>
      <div class="row">
        <button type="button" class="btn danger" onclick={removeFiles}>Delete</button>
        <button type="button" class="btn quiet" onclick={() => (removingFiles = false)}>Keep</button>
      </div>
    {:else}
      <div class="row">
        <button type="button" class="btn quiet danger" onclick={() => (removingAll = true)}>Remove everything…</button>
        {#if search.Installed}<button type="button" class="btn quiet" onclick={() => (removingFiles = true)}>Delete downloaded files only…</button>{/if}
      </div>
    {/if}
  </section>
{/if}

<style>
  .block {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 22px 0;
    border-top: 1px solid var(--line);
  }
  .block:first-of-type {
    padding-top: 0;
    border-top: 0;
  }
  .block h3 {
    margin: 0;
    font: 600 13.5px/1.3 var(--sans);
  }
  .callout {
    padding: 16px;
    border: 1px solid rgba(230, 232, 236, 0.45);
    border-radius: 12px;
    background: var(--lift);
  }
  .top {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .who {
    flex: 1;
    min-width: 0;
  }
  .dot {
    flex: none;
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--text-3);
  }
  .dot.on {
    background: var(--chrome);
    box-shadow: 0 0 0 3px rgba(230, 232, 236, 0.15);
  }
  .dot.bad {
    background: var(--danger);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .remove {
    padding-bottom: 0;
  }
  .remove .row {
    margin-left: -14px; /* the quiet buttons' own padding, so their words line up with the page */
  }
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
    transform-origin: left;
    transition: transform 0.4s var(--ease);
  }
  summary {
    width: fit-content;
    color: var(--text);
    font: 600 13.5px/1.3 var(--sans);
    cursor: pointer;
  }
  summary::marker {
    color: var(--text-3);
  }
  .options {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-top: 14px;
  }
  .draft {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 14px;
    border: 1px solid rgba(230, 232, 236, 0.45);
    border-radius: 10px;
    background: var(--lift);
  }
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
    margin-top: 8px;
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
