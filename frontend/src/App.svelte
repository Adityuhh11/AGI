<script lang="ts">
  import { onMount, tick } from "svelte";
  import { Clipboard } from "@wailsio/runtime";
  import { Service as Files } from "../bindings/agi/internal/files";
  import { Service as Panel } from "../bindings/agi/internal/panel";
  import { Service as Search } from "../bindings/agi/internal/search";
  import type { Results } from "../bindings/agi/internal/search";
  import { Service as Status } from "../bindings/agi/internal/status";
  import type { Info } from "../bindings/agi/internal/status";
  import ResultRow from "./lib/ResultRow.svelte";
  import { flatten, openLine, pathOf, step, type Item } from "./lib/flatten";

  const DEBOUNCE_MS = 120;

  let info = $state<Info | null>(null);
  let results = $state<Results | null>(null);
  let query = $state("");
  let selected = $state(0);
  let expanded = $state(new Set<string>());
  let problem = $state("");
  let input: HTMLInputElement;
  let main: HTMLElement;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let token = 0;

  const items = $derived<Item[]>(flatten(results?.Rows ?? [], expanded));
  const current = $derived<Item | undefined>(items[selected]);

  // The panel is only cloaked when closed, so the page keeps running: refocus and select the old
  // text every time the window regains the keyboard, so typing replaces it.
  function takeFocus() {
    input.focus();
    input.select();
    Status.Info().then((i) => (info = i));
  }

  function onInput() {
    problem = "";
    clearTimeout(timer);
    timer = setTimeout(runSearch, DEBOUNCE_MS);
  }

  async function runSearch() {
    const mine = ++token;
    const answer = await Search.Search(query, mine);
    if (answer.Stale || mine !== token) return; // a newer search is on its way
    results = answer;
    selected = 0;
    expanded = new Set();
  }

  async function open(item: Item | undefined, reveal = false) {
    if (!item) return;
    try {
      if (reveal) await Files.Reveal(pathOf(item));
      else await Files.Open(pathOf(item), openLine(item));
    } catch (error) {
      problem = String((error as Error)?.message ?? error);
    }
  }

  function toggle(open: boolean) {
    const row = current?.row;
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
    switch (event.key) {
      case "Escape":
        event.preventDefault();
        Panel.Hide();
        break;
      case "ArrowDown":
      case "ArrowUp":
        event.preventDefault();
        selected = step(selected, event.key === "ArrowDown" ? 1 : -1, items.length);
        scrollToSelected();
        break;
      case "Enter":
        event.preventDefault();
        open(current, event.ctrlKey);
        break;
      case "ArrowRight":
        if (atEndOfText() && current) {
          event.preventDefault();
          toggle(true);
        }
        break;
      case "ArrowLeft":
        if (atEndOfText() && current && (current.kind === "match" || expanded.has(current.row.Path))) {
          event.preventDefault();
          toggle(false);
        }
        break;
      case "C":
      case "c":
        if (event.ctrlKey && event.shiftKey && current) {
          event.preventDefault();
          Clipboard.SetText(pathOf(current));
        }
        break;
    }
  }

  async function scrollToSelected() {
    await tick();
    document.querySelector(".item.selected")?.scrollIntoView({ block: "nearest" });
  }

  // Fit the panel to what is on screen: the bar alone, or the bar plus the list. The observer
  // fires after layout, whenever the content changes size (results arriving, fonts loading).
  onMount(() => {
    takeFocus();
    const observer = new ResizeObserver(() => Panel.Resize(Math.ceil(main.getBoundingClientRect().height)));
    observer.observe(main);
    return () => observer.disconnect();
  });
</script>

<svelte:window onfocus={takeFocus} onkeydown={onKeydown} />

<main bind:this={main}>
  <input
    bind:this={input}
    bind:value={query}
    oninput={onInput}
    type="text"
    placeholder="Search"
    spellcheck="false"
    autocomplete="off"
  />

  {#if info?.HotkeyError}
    <p class="note problem">{info.HotkeyError}</p>
  {/if}
  {#if problem}
    <p class="note problem">{problem}</p>
  {/if}

  {#if query.trim() === ""}
    {#if info && info.Engine !== "ready" && info.Engine !== ""}
      <p class="note">
        {info.Engine === "failed" ? info.EngineMessage : "Search is starting up..."}
      </p>
    {/if}
  {:else if results?.State === "warming"}
    <p class="note">{results.Message}</p>
  {:else if results?.State === "unavailable" || results?.State === "error"}
    <p class="note problem">{results.Message}</p>
  {:else if results}
    {#if items.length === 0}
      <p class="note">No matches</p>
    {:else}
      {#if !results.Confident}
        <p class="note">No strong match. The closest files:</p>
      {/if}
      <div class="list">
        {#each items as item, index (item.kind === "row" ? item.row.Path : item.row.Path + "#" + item.match.Line)}
          <ResultRow
            row={item.row}
            match={item.kind === "match" ? item.match : undefined}
            selected={index === selected}
            expanded={expanded.has(item.row.Path)}
            onhover={() => (selected = index)}
            onactivate={() => open(item)}
          />
        {/each}
      </div>
    {/if}
  {/if}
</main>

<style>
  :global(html, body) {
    margin: 0;
    background: #000;
    color: #eee;
    font-family: "Segoe UI", system-ui, sans-serif;
    overflow: hidden;
  }
  input {
    box-sizing: border-box;
    display: block;
    width: 100%;
    padding: 22px 24px;
    border: 0;
    outline: 0;
    background: #000;
    color: #fff;
    font-size: 22px;
  }
  .note {
    margin: 0;
    padding: 6px 24px 12px;
    color: #777;
    font-size: 12.5px;
  }
  .problem {
    color: #c66;
  }
  .list {
    max-height: 540px;
    overflow-y: auto;
    padding-bottom: 8px;
    border-top: 1px solid #1c1c1c;
  }
  .list::-webkit-scrollbar {
    width: 8px;
  }
  .list::-webkit-scrollbar-thumb {
    background: #2a2a2a;
    border-radius: 4px;
  }
</style>
