<script lang="ts">
  import { onMount, tick } from "svelte";
  import { fade } from "svelte/transition";
  import { Clipboard } from "@wailsio/runtime";
  import { Service as Files } from "../bindings/agi/internal/files";
  import { Service as Palette } from "../bindings/agi/internal/palette";
  import type { Item as Entry } from "../bindings/agi/internal/palette";
  import { Service as Panel } from "../bindings/agi/internal/panel";
  import { Service as Search } from "../bindings/agi/internal/search";
  import type { Results } from "../bindings/agi/internal/search";
  import { Service as Status } from "../bindings/agi/internal/status";
  import type { Info } from "../bindings/agi/internal/status";
  import ActionPanel from "./lib/ActionPanel.svelte";
  import AppRow from "./lib/AppRow.svelte";
  import SearchGlyph from "./lib/SearchGlyph.svelte";
  import LiquidRing from "./lib/LiquidRing.svelte";
  import Loader from "./lib/Loader.svelte";
  import { METALS, nextMetal } from "./lib/palettes";
  import ResultRow from "./lib/ResultRow.svelte";
  import { actionsFor, type Action } from "./lib/actions";
  import { flatten, openLine, pathOf, step, type FileItem, type Item } from "./lib/flatten";
  import { cycle, escapeStep, label, type Mode } from "./lib/modes";

  const DEBOUNCE_MS = 120;
  const RING = 2.5; // px of metal around the card
  const MARGIN_TOP = 12; // clear window above and below the card, for its shadow
  const MARGIN_BOTTOM = 40;
  const BAR_HEIGHT = 60;
  const MORPH_MS = 240; // how long the card takes to grow or shrink (keep in step with the CSS)

  let info = $state<Info | null>(null);
  let results = $state<Results | null>(null);
  let query = $state("");
  let answered = $state(""); // the query `results` answers, so emphasis does not flicker while typing
  let selected = $state(0);
  let expanded = $state(new Set<string>());
  let problem = $state("");
  let active = $state(false); // the panel has the keyboard
  let metal = $state(0); // which of METALS the ring is made of; changes whenever the panel closes
  let pending = $state(false); // a search is in flight
  let typing = $state(false); // a key was pressed in the last moment
  let target = $state<Item | null>(null); // the row the open action panel acts on (null: panel closed)
  let mode = $state<Mode>("all"); // the chip: what a search covers, cycled with Tab
  let effective = $state<Mode>("all"); // the mode of the text once a prefix such as "app:" is taken off
  let entries = $state<Entry[]>([]); // the instant answers (apps): they arrive in a millisecond, before the files
  let instant = 0;
  let input: HTMLInputElement;
  let content: HTMLElement;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let token = 0;

  // Beside apps, files without a confident match are probably noise, so they are not shown at all.
  const fileRows = $derived(results && (results.Confident || entries.length === 0) ? (results.Rows ?? []) : []);
  const items = $derived<Item[]>(flatten(fileRows, expanded, entries));
  const current = $derived<Item | undefined>(items[selected]);
  const sections = $derived(entries.length > 0 && fileRows.length > 0); // two kinds of rows: label them
  const chip = $derived(results?.Scope || label[effective]);

  // What the page waits for, in a word or two, shown as the loader in the bar.
  const waiting = $derived.by(() => {
    if (info && info.Engine !== "ready" && info.Engine !== "failed" && info.Engine !== "") return "Warming up";
    if (info?.IndexState === "indexing") return "Indexing";
    return "";
  });

  // One line each, most urgent first, shown above the results.
  const notes = $derived.by(() => {
    const out: { text: string; problem: boolean }[] = [];
    if (info?.HotkeyError) out.push({ text: info.HotkeyError, problem: true });
    if (problem) out.push({ text: problem, problem: true });
    if (query.trim() === "") {
      if (info?.Engine === "failed") out.push({ text: info.EngineMessage, problem: true });
    } else if (effective === "apps") {
      if (entries.length === 0) out.push({ text: `No app matched “${query.replace(/^\s*apps?:/i, "").trim()}”`, problem: false });
    } else if (results?.State === "warming") {
      out.push({ text: results.Message, problem: false });
    } else if (results?.State === "unavailable" || results?.State === "error") {
      out.push({ text: results.Message, problem: true });
    } else if (results) {
      if (items.length === 0 && results.Message) out.push({ text: results.Message, problem: false });
      else if (items.length === 0 && answered.trim() !== "") out.push({ text: `Nothing matched “${answered}”`, problem: false });
      else if (!results.Confident && entries.length === 0) out.push({ text: "No strong match. The closest files:", problem: false });
    }
    return out;
  });
  const showPanel = $derived(notes.length > 0 || items.length > 0);

  // The card is exactly as tall as what it holds, plus its ring; the window around it is taller by the
  // shadow margins. The card animates to its new height in CSS; the window jumps to the larger of the
  // old and new height at once (so nothing is clipped mid-motion) and shrinks after the motion.
  let contentHeight = $state(0);
  const cardHeight = $derived(contentHeight > 0 ? Math.ceil(contentHeight + RING * 2) : BAR_HEIGHT + RING * 2);
  const windowHeight = $derived(cardHeight + MARGIN_TOP + MARGIN_BOTTOM);
  let appliedHeight = 0;
  let shrinkTimer: ReturnType<typeof setTimeout> | undefined;

  $effect(() => {
    const height = windowHeight;
    clearTimeout(shrinkTimer);
    if (height >= appliedHeight) resizeWindow(height);
    else shrinkTimer = setTimeout(() => resizeWindow(height), MORPH_MS + 40);
  });

  function resizeWindow(height: number) {
    appliedHeight = height;
    Panel.Resize(height);
  }

  $effect(() => {
    document.documentElement.dataset.active = String(active);
  });

  // The panel is only cloaked when closed, so the page keeps running: refocus and select the old
  // text every time the window regains the keyboard, so typing replaces it.
  function takeFocus() {
    active = true;
    input.focus();
    input.select();
    refreshInfo();
  }

  // The panel lost the keyboard, which is also when it closes: choose the next metal now, unseen.
  function onBlur() {
    active = false;
    closeActions();
    mode = "all"; // the next opening starts from everything
    effective = "all";
    metal = nextMetal(metal);
  }

  const refreshInfo = () => Status.Info().then((i) => (info = i));

  let typingTimer: ReturnType<typeof setTimeout> | undefined;

  function onInput() {
    problem = "";
    typing = true; // the ring flows faster while you type, and settles a moment after you stop
    clearTimeout(typingTimer);
    typingTimer = setTimeout(() => (typing = false), 700);
    clearTimeout(timer);
    if (query.trim() === "") {
      // Nothing to search for: drop the old answer at once, so the card closes back into the capsule
      // and no half-answer ("nothing matched") flashes while the next word is typed.
      token++;
      instant++;
      pending = false;
      results = null;
      answered = "";
      entries = [];
      effective = mode;
      return;
    }
    update();
  }

  // Asks Go for the instant answers (apps), which come back in a millisecond, and then lets the files
  // follow after the usual pause. In the Apps mode there is no file search.
  async function update() {
    const mine = ++instant;
    const answer = await Palette.Query(query, mode);
    if (mine !== instant) return; // a newer keystroke is already being answered
    entries = answer.Items ?? [];
    effective = answer.Mode as Mode;
    selected = 0;
    clearTimeout(timer);
    if (effective === "apps") {
      token++;
      pending = false;
      results = null;
      answered = "";
      return;
    }
    timer = setTimeout(runSearch, DEBOUNCE_MS);
  }

  function setMode(next: Mode) {
    mode = next;
    effective = next;
    if (query.trim() !== "") update();
  }

  async function runSearch() {
    const mine = ++token;
    const asked = query;
    pending = true;
    const answer = await Search.Search(asked, mine);
    if (answer.Stale || mine !== token) return; // a newer search is on its way
    pending = false;
    results = answer;
    answered = asked;
    selected = 0;
    expanded = new Set();
  }

  // The panel acts on the row that was selected when it opened, even if the pointer wanders over
  // others while it is open.
  function openActions() {
    if (current) target = current;
  }

  async function closeActions() {
    target = null;
    await tick();
    input?.focus();
  }

  async function runAction(id: Action["id"]) {
    const item = target;
    await closeActions();
    if (!item) return;
    if (id === "copy-path" && item.kind !== "app") copyPath(item);
    else if (id !== "copy-path") run(item, id);
  }

  function copyPath(item: FileItem) {
    Clipboard.SetText(pathOf(item));
    Panel.Hide(); // copied to be pasted elsewhere: give the keyboard back
  }

  /** Whether the row can be revealed or run as administrator (files can be revealed; apps say what they can). */
  function available(item: Item, how: "reveal" | "admin"): boolean {
    if (item.kind !== "app") return how === "reveal";
    return item.entry.Actions?.includes(how) ?? false;
  }

  async function run(item: Item | undefined, how: "open" | "reveal" | "admin" = "open") {
    if (!item) return;
    if (how !== "open" && !available(item, how)) return;
    try {
      if (item.kind === "app") await Palette.Do(item.entry.ID, how, query);
      else if (how === "reveal") await Files.Reveal(pathOf(item));
      else await Files.Open(pathOf(item), openLine(item));
    } catch (error) {
      problem = String((error as Error)?.message ?? error);
      if (item.kind === "app") update(); // an app that is gone has left the list
    }
  }

  function toggle(open: boolean) {
    const row = current && current.kind !== "app" ? current.row : undefined;
    if (!row || (row.Matches?.length ?? 0) < 2) return;
    const next = new Set(expanded);
    if (open) next.add(row.Path);
    else {
      next.delete(row.Path);
      if (current?.kind === "match") selected = items.findIndex((i) => i.kind === "row" && i.row === row);
    }
    expanded = next;
  }

  function atEndOfText() {
    return input.selectionStart === input.value.length && input.selectionEnd === input.value.length;
  }

  function onKeydown(event: KeyboardEvent) {
    if (target) return; // the action panel has the keyboard
    switch (event.key) {
      case "Escape": {
        event.preventDefault();
        const next = escapeStep(mode, query);
        if (next === "mode") setMode("all");
        else if (next === "text") {
          query = "";
          onInput();
        } else Panel.Hide();
        break;
      }
      case "Tab":
        if (!event.ctrlKey && !event.altKey) {
          event.preventDefault();
          setMode(cycle(mode, event.shiftKey ? -1 : 1));
        }
        break;
      case "ArrowDown":
      case "ArrowUp":
        event.preventDefault();
        selected = step(selected, event.key === "ArrowDown" ? 1 : -1, items.length);
        scrollToSelected();
        break;
      case "Enter":
        event.preventDefault();
        run(current, event.ctrlKey && event.shiftKey ? "admin" : event.ctrlKey ? "reveal" : "open");
        break;
      case "ArrowRight":
        if (atEndOfText() && current) {
          event.preventDefault();
          toggle(true);
        }
        break;
      case "ArrowLeft":
        if (atEndOfText() && current && current.kind !== "app" && (current.kind === "match" || expanded.has(current.row.Path))) {
          event.preventDefault();
          toggle(false);
        }
        break;
      case "C":
      case "c":
        if (event.ctrlKey && event.shiftKey && current && current.kind !== "app") {
          event.preventDefault();
          copyPath(current);
        }
        break;
      case "K":
      case "k":
        if (event.ctrlKey && current) {
          event.preventDefault();
          openActions();
        }
        break;
    }
  }

  async function scrollToSelected() {
    await tick();
    document.querySelector(".item.selected")?.scrollIntoView({ block: "nearest" });
  }

  const optionId = (index: number) => `option-${index}`;
  const itemKey = (item: Item) =>
    item.kind === "app" ? item.entry.ID : item.kind === "row" ? item.row.Path : item.row.Path + "#" + item.match.Line;

  onMount(() => {
    takeFocus();
    // The observer fires after layout, whenever what the card holds changes size (results arriving,
    // fonts loading); the card and the window follow.
    const observer = new ResizeObserver(([entry]) => (contentHeight = entry.borderBoxSize[0].blockSize));
    observer.observe(content);
    // While something is loading the numbers move; keep the bar honest without waiting for a refocus.
    const poll = setInterval(() => active && waiting && refreshInfo(), 1500);
    return () => {
      observer.disconnect();
      clearInterval(poll);
    };
  });
</script>

<svelte:window onfocus={takeFocus} onblur={onBlur} onkeydown={onKeydown} />

<main>
  <div class="card" style:height="{cardHeight}px">
    <LiquidRing speed={typing || pending ? 1.5 : 0.6} running={active} metal={METALS[metal]} />
    <div class="surface">
      <div class="content" class:acting={target} bind:this={content}>
        <header class="bar">
          <span class="glass"><SearchGlyph size={22} busy={pending} /></span>
          <input
            bind:this={input}
            bind:value={query}
            oninput={onInput}
            type="text"
            placeholder="Search apps and files"
            spellcheck="false"
            autocomplete="off"
            role="combobox"
            aria-label="Search apps and files"
            aria-expanded={showPanel}
            aria-controls="results"
            aria-activedescendant={current ? optionId(selected) : undefined}
          />
          {#if waiting}
            <span class="status" transition:fade={{ duration: 160 }}><Loader text={waiting} /></span>
          {:else if chip}
            <span class="status scope">{chip}</span>
          {/if}
        </header>

        {#if showPanel}
          <div class="panel" in:fade={{ duration: 160, delay: 60 }}>
            {#each notes as note}
              <p class="note" class:problem={note.problem}>{note.text}</p>
            {/each}
            {#if items.length > 0}
              <div class="results" class:stale={pending} id="results" role="listbox" aria-label="Results">
                {#each items as item, index (itemKey(item))}
                  {#if sections && index === 0}<h3 class="section">Apps</h3>{/if}
                  {#if sections && index === entries.length}<h3 class="section">Files</h3>{/if}
                  {#if item.kind === "app"}
                    <AppRow
                      id={optionId(index)}
                      entry={item.entry}
                      selected={index === selected}
                      onhover={() => (selected = index)}
                      onactivate={() => run(item)}
                    />
                  {:else}
                    <ResultRow
                      id={optionId(index)}
                      row={item.row}
                      match={item.kind === "match" ? item.match : undefined}
                      query={answered}
                      selected={index === selected}
                      expanded={expanded.has(item.row.Path)}
                      onhover={() => (selected = index)}
                      onactivate={() => run(item)}
                    />
                  {/if}
                {/each}
              </div>
              <footer class="hints">
                <span><kbd>↑↓</kbd>move</span>
                <span><kbd>Enter</kbd>open</span>
                {#if current && available(current, "reveal")}<span><kbd>Ctrl+Enter</kbd>reveal</span>{/if}
                <span><kbd>Ctrl+K</kbd>actions</span>
                {#if current && current.kind !== "app" && current.row.Matches && current.row.Matches.length > 1}
                  <span><kbd>→</kbd>more</span>
                {/if}
                <span><kbd>Tab</kbd>mode</span>
                <span><kbd>Esc</kbd>close</span>
                {#if info && info.IndexState && info.IndexState !== "uptodate"}
                  <span class="index">{info.IndexText}</span>
                {/if}
              </footer>
            {/if}
          </div>
        {/if}
      </div>
      {#if target}
        <ActionPanel actions={actionsFor(target)} onrun={runAction} onclose={closeActions} />
      {/if}
    </div>
  </div>
  <p class="live" role="status" aria-live="polite">
    {#if results && items.length > 0}{items.length} results{/if}
  </p>
</main>

<style>
  main {
    box-sizing: border-box;
    width: 100vw;
    padding: 12px 20px 40px;
  }
  .card {
    position: relative;
    border-radius: var(--radius);
    /* The ring's fallback and its first frame: static chrome, in case the shader cannot run. */
    background: linear-gradient(135deg, #565963, #e4e8f0 45%, #565963 70%, #c9ced9);
    box-shadow: var(--shadow);
    /* Height is animated on purpose: one small element, its content is absolutely positioned inside,
       and it only runs for 240 ms when results appear or go. */
    transition: height 240ms var(--ease);
  }
  .surface {
    position: absolute;
    inset: var(--ring);
    border-radius: calc(var(--radius) - var(--ring));
    background: var(--plate);
    overflow: hidden;
    isolation: isolate;
  }
  .surface::after {
    content: "";
    position: absolute;
    inset: 0;
    border-radius: inherit;
    pointer-events: none;
    /* The edge: a lit top rim and a hairline all round, as on a machined card. */
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.13),
      inset 0 0 0 1px rgba(255, 255, 255, 0.045);
  }
  .content {
    position: relative;
    z-index: 1;
  }
  /* Room for the action panel above the footer, however few rows there are. */
  .content.acting {
    min-height: 270px;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: 14px;
    height: 60px;
    padding: 0 24px 0 24px;
  }
  .glass {
    flex: none;
    display: grid;
    color: var(--text-3);
    transition: color 200ms var(--ease);
  }
  .bar:focus-within .glass {
    color: var(--chrome);
  }
  input {
    flex: 1;
    min-width: 0;
    height: 100%;
    padding: 0;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--text);
    font: 400 19px/1 var(--display);
    letter-spacing: -0.01em;
  }
  input::placeholder {
    color: var(--text-3);
  }
  .status {
    flex: none;
    margin-left: auto;
  }
  .scope {
    padding: 3px 11px;
    border-radius: 999px;
    background: var(--tile);
    color: var(--text-2);
    font: 500 11.5px var(--mono);
  }
  .panel {
    border-top: 1px solid var(--line);
  }
  .note {
    margin: 0;
    padding: 12px 26px 4px;
    color: var(--text-2);
    font-size: 12.5px;
  }
  .note.problem {
    color: var(--danger);
  }
  .results {
    max-height: 440px;
    overflow-y: auto;
    padding: 8px 0;
  }
  .results {
    transition: opacity 120ms var(--ease);
  }
  .results.stale {
    opacity: 0.55; /* the answer to an older query, until the new one lands */
  }
  .section {
    margin: 4px 24px 2px;
    color: var(--text-3);
    font: 500 10.5px var(--mono);
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }
  .section:not(:first-child) {
    margin-top: 10px;
  }
  .results::-webkit-scrollbar {
    width: 10px;
  }
  .results::-webkit-scrollbar-thumb {
    border: 3px solid transparent;
    border-radius: 999px;
    background: rgba(255, 255, 255, 0.14) padding-box;
  }
  .hints {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 11px 24px 13px;
    border-top: 1px solid var(--line);
    color: var(--text-3);
    font-size: 11.5px;
    white-space: nowrap;
  }
  kbd {
    margin-right: 7px;
    padding: 2px 7px;
    border-radius: 7px;
    background: var(--tile);
    color: var(--text-2);
    font: 500 10.5px var(--mono);
  }
  .index {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    text-align: right;
    font-family: var(--mono);
    font-size: 11px;
  }
  .live {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
</style>
