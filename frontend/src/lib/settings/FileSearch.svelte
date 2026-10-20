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

  // The words are the measurements in docs/eval/2026-10-18-pace.md: 17.6 s, 33.4 s and 103.5 s to index the same
  // folder, with the graphics card busy 65%, 27% and 16% of the time.
  const SPEEDS = [
    { id: "fast", name: "Fast", how: "Finishes soonest. The graphics card works hard, and other things may slow down meanwhile." },
    { id: "balanced", name: "Balanced", how: "Takes about twice as long. The graphics card is busy about a third of the time." },
    { id: "gentle", name: "Gentle", how: "Takes about six times as long. Your computer stays free and search stays quick." },
  ];
  const folderText = $derived(Object.fromEntries((search?.Folders ?? []).map((f) => [f.Root, f.Text])));

  // What each kind of file is searched by (docs/superpowers/specs/2026-10-20-indexing-choices-design.md). Pictures are
  // what indexing time goes on: about 0.16 s each on a graphics card, about 5 s on a processor alone.
  type Kind = "PDFPages" | "Photos" | "Videos";
  const CHOICES: { kind: Kind; label: string; options: { id: string; name: string; how: string }[] }[] = [
    {
      kind: "PDFPages",
      label: "PDF pages",
      options: [
        { id: "auto", name: "Pages that need it", how: "Scans, slides and pages with pictures or tables are also found by how they look. Every page is found by its words." },
        { id: "all", name: "Every page", how: "Any page can be found by how it looks. The slowest: every page is a picture to index." },
        { id: "text", name: "Words only", how: "Pages are found by their words. The fastest, and the choice for a computer without a graphics card." },
      ],
    },
    {
      kind: "Photos",
      label: "Photos",
      options: [
        { id: "content", name: "What they show", how: "Find a photo by describing it: “dog on a beach”." },
        { id: "name", name: "Their names", how: "Only the file name is searched. Much faster without a graphics card." },
      ],
    },
    {
      kind: "Videos",
      label: "Videos",
      options: [
        { id: "name", name: "Their names", how: "Videos are found by their file names." },
        { id: "content", name: "What is in them", how: "Eight frames of each video are searched too; a video costs as much as eight photos." },
      ],
    },
  ];

  async function choose(kind: Kind, id: string) {
    if (!search) return;
    const next = { PDFPages: search.PDFPages, Photos: search.Photos, Videos: search.Videos, [kind]: id };
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
      {#if search.Enabled && search.EngineState === "ready" && !search.NeedsUpdate}
        <p class="help">{search.Paused ? "Indexing is paused. Search still works on what is indexed." : search.IndexText}</p>
        <div>
          <button type="button" class="btn" onclick={() => (search?.Paused ? Settings.ResumeIndexing() : Settings.PauseIndexing()).then(refresh)}>
            {search.Paused ? "Resume indexing" : "Pause indexing"}
          </button>
        </div>
      {/if}
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

{#if search?.Installed}
  <div class="field">
    <span class="label" id="speed-label">How hard indexing works</span>
    <div class="speeds" role="radiogroup" aria-labelledby="speed-label">
      {#each SPEEDS as speed (speed.id)}
        <button
          type="button"
          role="radio"
          aria-checked={search.Pace === speed.id}
          class="speed"
          class:on={search.Pace === speed.id}
          onclick={() => run(() => Settings.SetPace(speed.id))}
        >
          <span class="name">{speed.name}</span>
          <span class="how">{speed.how}</span>
        </button>
      {/each}
    </div>
    <p class="help">Measured on a laptop with a 6 GB card. The index is built once; after that only what changes is read.</p>
  </div>

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
      <div class="speeds" style:--count={choice.options.length} role="radiogroup" aria-labelledby="{choice.kind}-label">
        {#each choice.options as option (option.id)}
          <button
            type="button"
            role="radio"
            aria-checked={search[choice.kind] === option.id}
            class="speed"
            class:on={search[choice.kind] === option.id}
            onclick={() => choose(choice.kind, option.id)}
          >
            <span class="name">{option.name}</span>
            <span class="how">{option.how}</span>
          </button>
        {/each}
      </div>
    {/each}
    <p class="help">A change applies at the next sync, to that kind of file only; nothing already indexed is computed again.</p>
  </div>
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
  .speeds {
    display: grid;
    grid-template-columns: repeat(var(--count, 3), minmax(0, 1fr));
    gap: 8px;
  }
  .sublabel {
    margin-top: 6px;
    color: var(--text-2);
    font-size: 12px;
  }
  .warn {
    color: var(--danger);
  }
  .speed {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 12px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: transparent;
    color: var(--text-2);
    text-align: left;
    cursor: pointer;
  }
  .speed:hover {
    background: var(--tile);
  }
  .speed.on {
    border-color: rgba(230, 232, 236, 0.45);
    background: var(--lift);
    color: var(--text);
  }
  .speed:focus-visible {
    outline: 1px solid var(--chrome);
    outline-offset: 2px;
  }
  .name {
    font: 620 13px/1.2 var(--sans);
  }
  .how {
    font-size: 11.5px;
    line-height: 1.45;
    color: var(--text-3);
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
