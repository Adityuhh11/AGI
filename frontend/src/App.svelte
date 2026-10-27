<script lang="ts">
  import { onMount, tick } from "svelte";
  import { fade } from "svelte/transition";
  import { Clipboard, Events } from "@wailsio/runtime";
  import { Service as Files } from "../bindings/agi/internal/files";
  import { Service as Palette } from "../bindings/agi/internal/palette";
  import type { Item as Entry } from "../bindings/agi/internal/palette";
  import { Service as Panel } from "../bindings/agi/internal/panel";
  import { Service as Search } from "../bindings/agi/internal/search";
  import { Service as SettingsWindow } from "../bindings/agi/internal/settingsui";
  import type { Results } from "../bindings/agi/internal/search";
  import { Service as Status } from "../bindings/agi/internal/status";
  import type { Info } from "../bindings/agi/internal/status";
  import ActionPanel from "./lib/ActionPanel.svelte";
  import EntryRow from "./lib/EntryRow.svelte";
  import Hints from "./lib/Hints.svelte";
  import SearchGlyph from "./lib/SearchGlyph.svelte";
  import LiquidRing from "./lib/LiquidRing.svelte";
  import Loader from "./lib/Loader.svelte";
  import { METALS, nextMetal } from "./lib/palettes";
  import ResultRow from "./lib/ResultRow.svelte";
  import { actionsFor, available, type Way } from "./lib/actions";
  import { filePathOf, flatten, openLine, pathOf, pinIdOf, step, tagIdOf, type Item } from "./lib/flatten";
  import { keyCommand } from "./lib/keys";
  import { asksForFiles, cycle, escapeStep, label, type Mode } from "./lib/modes";
  import { notesFor } from "./lib/notes";
  import TagPicker from "./lib/TagPicker.svelte";

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
  let picker = $state<{ id: string; title: string } | null>(null); // the thing the tag picker (Ctrl+T) is open on
  let tagsByID = $state<Record<string, string[] | null | undefined>>({}); // the tags of the rows on screen, shown as chips
  let answerText = $state(""); // the text the instant answer searched, with any prefix taken off
  let answerNote = $state(""); // why the Clipboard mode has nothing to list (it is off, nothing matches), if it has not
  let pins = $state(new Set<string>()); // the ids of everything pinned, to mark the rows
  let pinnedRows = $state<Entry[]>([]); // the pinned things that still exist, in the order pinned: the bar's empty state
  let keying = $state(false); // an arrow key moved the selection a moment ago: the list lifts the selected row and dims the rest
  let keyingTimer: ReturnType<typeof setTimeout> | undefined;
  const KEYING_MS = 900;
  let instant = 0;
  let input: HTMLInputElement;
  let content: HTMLElement;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let token = 0;

  // Beside apps, files without a confident match are probably noise, so they are not shown at all.
  const local = $derived(entries.filter((e) => e.Kind !== "web")); // answers about your own machine
  const fileRows = $derived(results && (results.Confident || local.length === 0) ? (results.Rows ?? []) : []);
  const showingPins = $derived(query.trim() === "" && mode === "all"); // nothing typed: the bar lists what is pinned
  const items = $derived<Item[]>(flatten(fileRows, expanded, entries, showingPins, info?.GroupResults ?? false));
  const current = $derived<Item | undefined>(items[selected]);
  const firstApp = $derived(items.findIndex((i) => i.kind === "entry" && i.entry.Kind === "app"));
  const firstFile = $derived(items.findIndex((i) => i.kind === "row"));
  const sections = $derived(firstApp >= 0 && firstFile >= 0); // two kinds of rows: label them
  const searchWhat = $derived(
    effective === "files" ? "Search your files" : info?.FilesInBar ? "Search apps, files and the web" : "Search apps and the web",
  );
  const chip = $derived(results?.Scope || label[effective]);

  // The tags of the rows on screen, for their chips.
  $effect(() => {
    const ids = items.map(tagIdOf).filter((id): id is string => id !== null);
    let fresh = true;
    if (ids.length === 0) tagsByID = {};
    else Palette.TagsOf(ids).then((found) => fresh && (tagsByID = found ?? {})); // Go sends null for none
    return () => (fresh = false);
  });
  const tagsOf = (item: Item) => tagsByID[tagIdOf(item) ?? ""] ?? [];

  // What the page waits for, in a word or two, shown as the loader in the bar.
  const waiting = $derived.by(() => {
    if (info && !["ready", "failed", "off", ""].includes(info.Engine)) return "Warming up";
    if (info?.IndexState === "indexing") return "Indexing";
    return "";
  });

  const notes = $derived(
    notesFor({
      info,
      problem,
      query,
      effective,
      entries: entries.length,
      local: local.length,
      answerNote,
      results,
      fileRows: fileRows.length,
      answered,
    }),
  );
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
    if (performance.now() - filledAt < FILL_GRACE_MS) input.setSelectionRange(input.value.length, input.value.length);
    else input.select();
    refreshInfo();
  }

  // A hotkey that opens the bar with text typed in ("clip: ") tells the page just before the bar is shown. The
  // window gets several focus events while it opens, so for a second after the text arrives each of them leaves
  // it alone, with the cursor after it, instead of selecting it for the next keystroke to replace.
  const FILL_GRACE_MS = 1000;
  let filledAt = -Infinity;
  $effect(() =>
    Events.On("panel:fill", (e) => {
      filledAt = performance.now();
      mode = "all";
      query = String(e.data ?? "");
      onInput();
      input?.focus();
      input?.setSelectionRange(query.length, query.length);
    }),
  );

  // The panel lost the keyboard, which is usually when it closes. But the window being activated sends a blur
  // of its own while the bar opens, and that must not wipe the text a hotkey typed into it ("clip: "), so
  // the bar counts as closed only if the keyboard has not come back a moment later.
  const CLOSE_CHECK_MS = 120;
  let closeTimer: ReturnType<typeof setTimeout> | undefined;
  function onBlur() {
    active = false;
    closeActions();
    picker = null;
    clearTimeout(closeTimer);
    closeTimer = setTimeout(() => !document.hasFocus() && closed(), CLOSE_CHECK_MS);
  }

  // The bar has closed: the next opening starts from everything, with the pinned things listed, and in the
  // next metal, chosen now, unseen.
  function closed() {
    mode = "all";
    effective = "all";
    query = "";
    clearAnswer();
    loadPins();
    metal = nextMetal(metal);
  }

  // What is pinned, for the empty bar and for the marks on rows. Asked again whenever it may have changed.
  async function loadPins() {
    const [ids, rows] = await Promise.all([Palette.PinIDs(), Palette.Pinned()]);
    pins = new Set(ids ?? []);
    pinnedRows = rows ?? [];
    if (showingPins) {
      entries = pinnedRows;
      selected = Math.min(selected, Math.max(0, entries.length - 1));
    }
  }

  async function togglePin(item: Item | undefined) {
    const id = item && pinIdOf(item);
    if (!id) return;
    await attempt(async () => {
      await Palette.TogglePin(id);
      await loadPins();
    });
  }

  const pinningOf = (item: Item) => {
    const id = pinIdOf(item);
    return id === null ? null : pins.has(id) ? "unpin" : "pin";
  };

  const refreshInfo = () => Status.Info().then((i) => (info = i));

  let typingTimer: ReturnType<typeof setTimeout> | undefined;

  function onInput() {
    problem = "";
    typing = true; // the ring flows faster while you type, and settles a moment after you stop
    clearTimeout(typingTimer);
    typingTimer = setTimeout(() => (typing = false), 700);
    clearTimeout(timer);
    if (query.trim() === "") {
      clearAnswer();
      return;
    }
    update();
  }

  // Nothing to search for: drop the old answer at once, so the card goes back to what is pinned (or closes
  // into the capsule) and no half-answer ("nothing matched") flashes while the next word is typed.
  function clearAnswer() {
    token++;
    instant++;
    pending = false;
    results = null;
    answered = "";
    entries = mode === "all" ? pinnedRows : [];
    selected = 0;
    answerText = "";
    answerNote = "";
    effective = mode;
  }

  // Asks Go for the instant answers (apps), which come back in a millisecond, and then lets the files
  // follow after the usual pause. In the Apps, Web and Tags modes there is no file search.
  async function update() {
    const mine = ++instant;
    const answer = await Palette.Query(query, mode);
    if (mine !== instant) return; // a newer keystroke is already being answered
    entries = answer.Items ?? [];
    effective = answer.Mode as Mode;
    answerText = answer.Text;
    answerNote = answer.Note ?? "";
    selected = 0;
    clearTimeout(timer);
    if ((effective === "all" && !info?.FilesInBar && !asksForFiles(query)) || effective === "apps" || effective === "web" || effective === "tags" || effective === "clip" || effective === "snip" || effective === "note") {
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
    else clearAnswer();
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

  async function runAction(id: string) {
    const item = target;
    await closeActions();
    if (!item) return;
    if (id === "copy-path") copyPath(item);
    else if (id === "tag") openPicker(item);
    else if (id === "pin") togglePin(item);
    else run(item, id as Way);
  }

  function copyPath(item: Item) {
    const path = filePathOf(item);
    if (!path) return;
    Clipboard.SetText(path);
    Panel.Hide(); // copied to be pasted elsewhere: give the keyboard back
  }

  // --- tags

  function openPicker(item: Item | undefined) {
    const id = item && tagIdOf(item);
    if (item && id) picker = { id, title: item.kind === "entry" ? item.entry.Title : item.row.Name };
  }

  async function closePicker() {
    picker = null;
    await tick();
    input?.focus();
  }

  function tagsChanged(id: string, tags: string[]) {
    tagsByID = { ...tagsByID, [id]: tags };
    if (mode === "tags" || query.trim().startsWith("#")) update(); // the list being looked at has changed
  }

  /** Runs something the user asked for; if it fails the reason is shown above the results. */
  async function attempt(action: () => Promise<unknown>, onFail?: () => void) {
    try {
      await action();
    } catch (error) {
      problem = String((error as Error)?.message ?? error);
      onFail?.();
    }
  }

  async function run(item: Item | undefined, how: Way = "open") {
    if (!item) return;
    if (how !== "open" && !available(item, how)) return;
    if (item.kind === "entry" && item.entry.Kind === "tag") {
      query = "#" + item.entry.ID.slice("tag:".length) + " ";
      onInput(); // show what carries it
      input.focus();
    } else if (item.kind === "entry" && item.entry.Fill) {
      query = item.entry.Fill; // a pinned quicklink or a built-in mode: the bar fills its trigger in and the rest is typed
      onInput();
      input.focus();
    } else if (item.kind === "entry") {
      const id = item.entry.ID;
      // An app that is gone has left the list, so the rows are asked for again.
      await attempt(() => Palette.Do(id, how, query), update);
      if (how === "delete") update(); // what was listed has changed
    } else if (how === "reveal") await attempt(() => Files.Reveal(pathOf(item)));
    else await attempt(() => Files.Open(pathOf(item), openLine(item)));
  }

  /** Alt+Enter: search the web for what is typed, from any mode. */
  function searchTheWeb() {
    const text = (answerText || query).trim();
    if (text) attempt(() => Palette.Do("web:" + text, "open", query));
  }

  function toggle(open: boolean) {
    const row = current && current.kind !== "entry" ? current.row : undefined;
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
    if (target || picker) return; // a panel has the keyboard
    const command = keyCommand(event, {
      current,
      atEndOfText: atEndOfText(),
      expanded: current !== undefined && current.kind !== "entry" && expanded.has(current.row.Path),
    });
    if (!command) return;
    event.preventDefault();
    switch (command.do) {
      case "settings":
        SettingsWindow.Open();
        Panel.Hide();
        break;
      case "escape": {
        const next = escapeStep(mode, query);
        if (next === "mode") setMode("all");
        else if (next === "text") {
          query = "";
          onInput();
        } else Panel.Hide();
        break;
      }
      case "mode":
        setMode(cycle(mode, command.by, info?.FileSearch));
        break;
      case "move":
        if (info?.SelectionEffect) {
          keying = true;
          clearTimeout(keyingTimer);
          keyingTimer = setTimeout(() => (keying = false), KEYING_MS);
        }
        selected = step(selected, command.by, items.length);
        scrollToSelected();
        break;
      case "web":
        searchTheWeb();
        break;
      case "run":
        run(current, command.how);
        break;
      case "expand":
      case "collapse":
        toggle(command.do === "expand");
        break;
      case "copy-path":
        if (current) copyPath(current);
        break;
      case "tag":
        openPicker(current);
        break;
      case "pin":
        togglePin(current);
        break;
      case "actions":
        openActions();
        break;
    }
  }

  // The list glides to the selected row: each frame covers a fixed share of what is left, so the motion is continuous at any
  // key-repeat rate. (CSS smooth scrolling restarts its animation on every repeat and moves in jerks while a key is held.)
  const GLIDE_MS = 70; // time constant: about 95% of the way in three of these
  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
  let glideFrame = 0;
  let glideGoal = 0;

  async function scrollToSelected() {
    await tick();
    const box = document.getElementById("results");
    const row = box?.querySelector<HTMLElement>(".item.selected");
    if (!box || !row) return;
    // Where the list is heading counts as where it is, so a second key during a glide aims from the first one's goal.
    const at = glideFrame ? glideGoal : box.scrollTop;
    const shift = box.scrollTop - at;
    const edge = 8;
    const view = box.getBoundingClientRect();
    const rect = row.getBoundingClientRect();
    let goal = at;
    if (selected === 0) goal = 0; // the first row brings its heading with it
    else if (rect.top + shift < view.top + edge) goal = at + (rect.top + shift - view.top - edge);
    else if (rect.bottom + shift > view.bottom - edge) goal = at + (rect.bottom + shift - view.bottom + edge);
    goal = Math.min(Math.max(goal, 0), box.scrollHeight - box.clientHeight);
    if (reduceMotion.matches) {
      box.scrollTop = goal;
      return;
    }
    glideTo(box, goal);
  }

  function glideTo(box: HTMLElement, goal: number) {
    glideGoal = goal;
    if (glideFrame) return; // the running loop takes the new goal
    let pos = box.scrollTop;
    let last = performance.now();
    const frame = (now: number) => {
      // Something else moved the list (the wheel, new results): that wins, the glide ends.
      if (!box.isConnected || Math.abs(box.scrollTop - pos) > 2) {
        glideFrame = 0;
        return;
      }
      pos += (glideGoal - pos) * (1 - Math.exp(-Math.min(now - last, 50) / GLIDE_MS));
      last = now;
      if (Math.abs(glideGoal - pos) < 0.5) pos = glideGoal;
      box.scrollTop = pos;
      glideFrame = pos === glideGoal ? 0 : requestAnimationFrame(frame);
    };
    glideFrame = requestAnimationFrame(frame);
  }

  const optionId = (index: number) => `option-${index}`;
  const itemKey = (item: Item) =>
    item.kind === "entry" ? item.entry.ID : item.kind === "row" ? item.row.Path : item.row.Path + "#" + item.match.Line;

  onMount(() => {
    takeFocus();
    loadPins();
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
      <div class="content" class:acting={target || picker} bind:this={content}>
        <header class="bar">
          <span class="glass"><SearchGlyph size={22} busy={pending} /></span>
          <input
            bind:this={input}
            bind:value={query}
            oninput={onInput}
            type="text"
            placeholder={searchWhat}
            spellcheck="false"
            autocomplete="off"
            role="combobox"
            aria-label={searchWhat}
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
              <div class="results" class:stale={pending} class:keying id="results" role="listbox" aria-label="Results">
                {#each items as item, index (itemKey(item))}
                  {#if showingPins && index === 0}<h3 class="section">Pinned</h3>{/if}
                  {#if sections && index === firstApp}<h3 class="section">Apps</h3>{/if}
                  {#if item.kind === "row" && item.heading}<h3 class="section">{item.heading}</h3>
                  {:else if sections && index === firstFile}<h3 class="section">Files</h3>{/if}
                  {#if item.kind === "entry"}
                    <EntryRow
                      id={optionId(index)}
                      entry={item.entry}
                      tags={tagsOf(item)}
                      pinned={pins.has(pinIdOf(item) ?? "")}
                      selected={index === selected}
                      onhover={() => ((selected = index), (keying = false))}
                      onactivate={() => run(item)}
                    />
                  {:else}
                    <ResultRow
                      id={optionId(index)}
                      row={item.row}
                      tags={tagsOf(item)}
                      pinned={pins.has(pinIdOf(item) ?? "")}
                      match={item.kind === "match" ? item.match : undefined}
                      query={answered}
                      selected={index === selected}
                      expanded={expanded.has(item.row.Path)}
                      onhover={() => ((selected = index), (keying = false))}
                      onactivate={() => run(item)}
                    />
                  {/if}
                {/each}
              </div>
              <Hints {current} pinned={current && pinningOf(current) ? pinningOf(current) === "unpin" : null} {info} />
            {/if}
          </div>
        {/if}
      </div>
      {#if target}
        <ActionPanel actions={actionsFor(target, tagIdOf(target) !== null, pinningOf(target))} onrun={runAction} onclose={closeActions} />
      {:else if picker}
        <TagPicker id={picker.id} title={picker.title} {attempt} onchange={tagsChanged} onclose={closePicker} />
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
  /* Rows are faded by the colour of their text, never by opacity or a transform: those make the browser draw the row as a
     picture that is then scaled or blended, which is what made it look soft. Dimmed text is the same glyphs in a fainter
     colour, so it stays as sharp as ever. */
  .results {
    --dim-text: color-mix(in srgb, var(--text) 45%, transparent);
    --dim-text-3: color-mix(in srgb, var(--text-3) 55%, transparent);
  }
  .results :global(.item) {
    transition:
      background-color 200ms var(--ease),
      box-shadow 240ms var(--ease);
  }
  .results :global(.item .name),
  .results :global(.item .kind),
  .results :global(.item .folder),
  .results :global(.item .detail),
  .results :global(.item .where),
  .results :global(.item mark),
  .results :global(.item .tag),
  .results :global(.item .pin),
  .results :global(.item .tile) {
    transition:
      color 200ms var(--ease),
      opacity 200ms var(--ease);
  }
  /* While a search is in flight the list shows the answer to an older query, a little fainter. While the arrow keys move the
     selection (and for a moment after) the selected row is a lit pane of glass and the rest fade back. */
  .results.stale :global(.item .name),
  .results.stale :global(.item mark),
  .results.keying :global(.item:not(.selected) .name),
  .results.keying :global(.item:not(.selected) mark) {
    color: var(--dim-text);
  }
  .results.stale :global(.item .kind),
  .results.stale :global(.item .folder),
  .results.stale :global(.item .detail),
  .results.stale :global(.item .where),
  .results.stale :global(.item .tag),
  .results.stale :global(.item .pin),
  .results.keying :global(.item:not(.selected) .kind),
  .results.keying :global(.item:not(.selected) .folder),
  .results.keying :global(.item:not(.selected) .detail),
  .results.keying :global(.item:not(.selected) .where),
  .results.keying :global(.item:not(.selected) .tag),
  .results.keying :global(.item:not(.selected) .pin) {
    color: var(--dim-text-3);
  }
  .results.keying :global(.item:not(.selected) .tile) {
    opacity: 0.5; /* the tile holds no text: only an icon */
  }
  .results.keying :global(.item.selected) {
    position: relative;
    z-index: 1;
    background-color: rgba(255, 255, 255, 0.115);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.2),
      0 0 0 1px rgba(255, 255, 255, 0.08),
      0 8px 16px -8px rgba(0, 0, 0, 0.95);
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
  .live {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
</style>
