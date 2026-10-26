<script lang="ts">
  import { onMount, tick } from "svelte";
  import { Browser } from "@wailsio/runtime";
  import { Service as Notes } from "../bindings/agi/internal/notes";
  import type { Note } from "../bindings/agi/internal/notes";
  import "./lib/settings/forms.css";
  import { when } from "./lib/format";
  import { render, toggleTask } from "./lib/markdown";

  const SAVE_AFTER_MS = 400;
  const PIN_KEY = "agi-notes-on-top";

  let list = $state<Note[]>([]);
  let query = $state("");
  let loaded = $state(false);
  let editing = $state<{ id: string; text: string } | null>(null); // the note in the editor; id is "" until its first save
  let preview = $state(false);
  let status = $state("");
  let onTop = $state(true);
  let confirmDelete = $state(false);
  let problem = $state("");
  let area = $state<HTMLTextAreaElement | undefined>();
  let search = $state<HTMLInputElement | undefined>();
  let timer: ReturnType<typeof setTimeout> | undefined;
  let saving: Promise<void> = Promise.resolve(); // saves run one after another, so a slow one never lands after a newer one

  const html = $derived(editing ? render(editing.text) : "");
  const words = $derived(editing ? editing.text.trim().split(/\s+/).filter(Boolean).length : 0);

  async function show(action: () => Promise<unknown>) {
    try {
      problem = "";
      await action();
    } catch (error) {
      problem = String((error as Error)?.message ?? error);
    }
  }

  async function loadList() {
    await show(async () => {
      list = (await Notes.List(query)) ?? [];
      loaded = true;
    });
  }

  function scheduleSave() {
    status = "";
    clearTimeout(timer);
    timer = setTimeout(flush, SAVE_AFTER_MS);
  }

  /** Saves what is in the editor now, after any save already under way. */
  function flush(): Promise<void> {
    clearTimeout(timer);
    const note = editing;
    if (!note) return saving;
    const text = note.text;
    saving = saving.then(() =>
      show(async () => {
        if (note.id === "" && text.trim() === "") return; // nothing typed yet: no note is made
        const saved = await Notes.Save(note.id, text);
        if (note.id === "" && saved.ID) note.id = saved.ID;
        if (editing === note) status = "Saved";
      }),
    );
    return saving;
  }

  async function open(id: string) {
    await flush();
    await show(async () => {
      const note = await Notes.Get(id);
      editing = { id, text: note.Text };
      preview = false;
      confirmDelete = false;
      status = "";
    });
    await tick();
    area?.focus();
  }

  async function create(text = "") {
    await flush();
    editing = { id: "", text };
    preview = false;
    confirmDelete = false;
    status = "";
    await tick();
    area?.focus();
  }

  async function back() {
    await flush();
    const note = editing;
    editing = null;
    confirmDelete = false;
    if (note && note.id && note.text.trim() === "") await show(() => Notes.Delete(note.id)); // an emptied note is not kept
    await loadList();
    await tick();
    search?.focus();
  }

  async function remove() {
    const note = editing;
    clearTimeout(timer);
    editing = null;
    confirmDelete = false;
    if (note?.id) await show(() => Notes.Delete(note.id));
    await loadList();
  }

  async function togglePin() {
    onTop = !onTop;
    Notes.SetOnTop(onTop);
    try {
      localStorage.setItem(PIN_KEY, String(onTop));
    } catch {
      // a window without storage forgets the choice; it still works
    }
  }

  function onPreviewClick(event: MouseEvent) {
    const target = event.target as HTMLElement;
    const link = target.closest("a");
    if (link) {
      event.preventDefault(); // a click must not navigate the notes window itself
      Browser.OpenURL(link.getAttribute("href") ?? "");
    } else if (target instanceof HTMLInputElement && target.dataset.line && editing) {
      editing.text = toggleTask(editing.text, Number(target.dataset.line));
      scheduleSave();
    }
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.ctrlKey && event.key.toLowerCase() === "n") {
      event.preventDefault();
      create();
    } else if (event.ctrlKey && event.key.toLowerCase() === "e" && editing) {
      event.preventDefault();
      preview = !preview;
    } else if (event.key === "Escape") {
      if (confirmDelete) confirmDelete = false;
      else if (editing) back();
      else if (query) {
        query = "";
        loadList();
      }
    }
  }

  // The address says what the window is for: #notes (the list), #notes/new, or #notes/<id>.
  async function follow() {
    const want = location.hash.split("/")[1] ?? "";
    if (want === "new") await create();
    else if (want !== "") await open(want);
    else if (editing) await back();
  }

  onMount(() => {
    document.documentElement.dataset.surface = "settings";
    try {
      onTop = localStorage.getItem(PIN_KEY) !== "false";
    } catch {
      onTop = true;
    }
    Notes.SetOnTop(onTop);
    loadList().then(follow);
    window.addEventListener("hashchange", follow);
    window.addEventListener("blur", flush);
    return () => {
      window.removeEventListener("hashchange", follow);
      window.removeEventListener("blur", flush);
    };
  });
</script>

<svelte:window onkeydown={onKeydown} />

<main>
  {#if editing}
    <header>
      <button type="button" class="btn quiet" onclick={back} title="Back to the notes (Esc)" aria-label="Back to the notes">←</button>
      <span class="state" aria-live="polite">{status || (editing.id === "" ? "New note" : "")}</span>
      <div class="tools">
        <button type="button" class="btn quiet" class:on={preview} onclick={() => (preview = !preview)} title="Edit or read (Ctrl+E)">
          {preview ? "Edit" : "Read"}
        </button>
        {#if confirmDelete}
          <button type="button" class="btn danger" onclick={remove}>Delete</button>
          <button type="button" class="btn quiet" onclick={() => (confirmDelete = false)}>Keep</button>
        {:else}
          <button type="button" class="btn quiet" onclick={() => (confirmDelete = true)} title="Delete this note">Delete</button>
        {/if}
        <button type="button" class="btn quiet" class:on={onTop} aria-pressed={onTop} onclick={togglePin} title="Keep above other windows">On top</button>
      </div>
    </header>
    {#if preview}
      <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
      <div class="read" onclick={onPreviewClick}>{@html html || '<p class="empty">Nothing here yet.</p>'}</div>
    {:else}
      <textarea
        bind:this={area}
        bind:value={editing.text}
        oninput={scheduleSave}
        spellcheck="true"
        placeholder="Write in markdown. # heading, **bold**, - [ ] task, `code`"
        aria-label="Note"
      ></textarea>
    {/if}
    <footer>{words} {words === 1 ? "word" : "words"}</footer>
  {:else}
    <header>
      <input
        bind:this={search}
        bind:value={query}
        oninput={loadList}
        type="text"
        placeholder="Search notes"
        aria-label="Search notes"
        spellcheck="false"
        autocomplete="off"
      />
      <div class="tools">
        <button type="button" class="btn primary" onclick={() => create()} title="New note (Ctrl+N)">New</button>
        <button type="button" class="btn quiet" class:on={onTop} aria-pressed={onTop} onclick={togglePin} title="Keep above other windows">On top</button>
      </div>
    </header>
    {#if problem}<p class="error pad">{problem}</p>{/if}
    {#if list.length > 0}
      <ul class="notes">
        {#each list as note (note.ID)}
          <li>
            <button type="button" onclick={() => open(note.ID)}>
              <span class="title">{note.Title}</span>
              <span class="when">{when(note.Modified)}</span>
              {#if note.Preview}<span class="preview">{note.Preview}</span>{/if}
            </button>
          </li>
        {/each}
      </ul>
    {:else if loaded}
      <p class="none">{query ? `No note has “${query}” in it.` : "No notes yet. Press New, or Ctrl+N, and start typing."}</p>
    {/if}
  {/if}
  {#if problem && editing}<p class="error pad">{problem}</p>{/if}
</main>

<style>
  main {
    display: flex;
    flex-direction: column;
    height: 100vh;
    max-width: none;
    background: var(--plate);
    color: var(--text);
  }
  header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    border-bottom: 1px solid var(--line);
  }
  header input {
    flex: 1;
    min-width: 0;
  }
  .tools {
    display: flex;
    gap: 4px;
    margin-left: auto;
  }
  .state {
    color: var(--text-3);
    font-size: 12px;
  }
  :global(main .btn.quiet.on) {
    color: var(--text);
    background: var(--lift);
  }
  textarea {
    flex: 1;
    min-height: 0;
    padding: 16px 18px;
    border: 0;
    outline: 0;
    resize: none;
    background: transparent;
    color: var(--text);
    font: 450 14px/1.6 var(--sans);
    tab-size: 2;
  }
  textarea::placeholder {
    color: var(--text-3);
  }
  footer {
    padding: 6px 14px;
    border-top: 1px solid var(--line);
    color: var(--text-3);
    font-size: 11.5px;
    font-variant-numeric: tabular-nums;
  }
  .notes {
    flex: 1;
    min-height: 0;
    margin: 0;
    padding: 6px 8px;
    overflow-y: auto;
    list-style: none;
  }
  .notes button {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: 2px 12px;
    width: 100%;
    padding: 10px 12px;
    border: 0;
    border-radius: 12px;
    background: transparent;
    color: inherit;
    text-align: left;
    cursor: pointer;
    transition: background-color 0.12s;
  }
  .notes button:hover {
    background: var(--tile);
  }
  .notes button:active {
    background: var(--lift);
  }
  .title {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font: 560 13.5px/1.35 var(--sans);
  }
  .when {
    color: var(--text-3);
    font-size: 11.5px;
    font-variant-numeric: tabular-nums;
    align-self: center;
  }
  .preview {
    grid-column: 1 / -1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-2);
    font-size: 12.5px;
  }
  .none {
    margin: 0;
    padding: 32px 24px;
    color: var(--text-3);
    font-size: 13px;
    text-align: center;
  }
  .pad {
    padding: 8px 14px 0;
  }
  .read {
    flex: 1;
    min-height: 0;
    padding: 14px 18px 18px;
    overflow-y: auto;
    font: 450 14px/1.6 var(--sans);
  }
  .read :global(h1),
  .read :global(h2),
  .read :global(h3),
  .read :global(h4),
  .read :global(h5),
  .read :global(h6) {
    margin: 1.1em 0 0.35em;
    font-weight: 620;
    letter-spacing: -0.01em;
    line-height: 1.25;
  }
  .read :global(h1) {
    font-size: 20px;
  }
  .read :global(h2) {
    font-size: 17px;
  }
  .read :global(h3),
  .read :global(h4),
  .read :global(h5),
  .read :global(h6) {
    font-size: 14.5px;
  }
  .read > :global(:first-child) {
    margin-top: 0;
  }
  .read :global(p),
  .read :global(ul),
  .read :global(ol),
  .read :global(blockquote),
  .read :global(pre) {
    margin: 0 0 0.8em;
  }
  .read :global(ul),
  .read :global(ol) {
    padding-left: 1.3em;
  }
  .read :global(li.task) {
    list-style: none;
    margin-left: -1.3em;
  }
  .read :global(li.task.done span) {
    color: var(--text-3);
    text-decoration: line-through;
  }
  .read :global(input[type="checkbox"]) {
    margin-right: 6px;
    accent-color: var(--chrome);
    cursor: pointer;
  }
  .read :global(blockquote) {
    padding: 2px 0 2px 14px;
    border-left: 1px solid var(--text-3);
    color: var(--text-2);
  }
  .read :global(code) {
    padding: 1px 5px;
    border-radius: 6px;
    background: var(--tile);
    font: 500 12.5px var(--mono);
  }
  .read :global(pre) {
    padding: 12px 14px;
    overflow-x: auto;
    border-radius: 10px;
    background: var(--tile);
  }
  .read :global(pre code) {
    padding: 0;
    background: none;
  }
  .read :global(a) {
    color: var(--chrome);
    text-underline-offset: 2px;
  }
  .read :global(hr) {
    border: 0;
    border-top: 1px solid var(--line);
    margin: 1.2em 0;
  }
  .read :global(.empty) {
    color: var(--text-3);
  }
</style>
