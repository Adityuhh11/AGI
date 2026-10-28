<script lang="ts">
  import { onMount, tick } from "svelte";
  import { Browser, Clipboard, Events, Window } from "@wailsio/runtime";
  import { Service as Chat } from "../bindings/omni/internal/chat";
  import type { Info, Summary, View } from "../bindings/omni/internal/chat";
  import "./lib/settings/forms.css";
  import { dot, escapeStep, lastQuestion, meter, sends, thoughtFor, unloadsIn, waiting, type Phase } from "./lib/chat/chat";
  import { copyable } from "./lib/chat/copy";
  import { when } from "./lib/format";
  import { render } from "./lib/markdown";

  const PIN_KEY = "omni-chat-on-top";
  const EMPTY: View = { id: "", title: "", messages: [], memory_start: 0, used: 0, room: 8192, unsaved: false, busy: false };

  let view = $state<View>(EMPTY);
  let info = $state<Info | null>(null);
  let draft = $state("");
  let error = $state("");
  // The answer being written: it is shown from the events, and replaced by the finished message when the answer ends.
  let live = $state<{ text: string; thinking: string; phase: Phase; status: string } | null>(null);
  let onTop = $state(true);
  let historyOpen = $state(false);
  let history = $state<Summary[]>([]);
  let historyQuery = $state("");
  let confirm = $state<string>(""); // the conversation whose delete is waiting for a second click, or "all"
  let opened = $state<Record<number, boolean>>({}); // which answers have their thinking shown
  let now = $state(Date.now());
  let box = $state<HTMLTextAreaElement | undefined>();
  let log = $state<HTMLDivElement | undefined>();
  let stuck = true; // the list follows the end of the answer unless the person scrolled up

  const messages = $derived(view.messages ?? []);
  const busy = $derived(view.busy || live !== null);
  const status = $derived(dot(info));
  const off = $derived(info !== null && (!info.enabled || !info.installed));
  const unload = $derived(info && info.state === "ready" ? unloadsIn(info.unloads_at, now) : "");
  const lastAnswer = $derived(messages.length > 0 && messages[messages.length - 1].role === "assistant" ? messages.length - 1 : -1);
  const canRetry = $derived(!busy && messages.length > 0 && !off);

  async function toEnd(force = false) {
    if (!force && !stuck) return;
    await tick();
    if (log) log.scrollTop = log.scrollHeight;
  }

  function onScroll() {
    if (log) stuck = log.scrollHeight - log.scrollTop - log.clientHeight < 48;
  }

  function grow() {
    if (!box) return;
    box.style.height = "auto";
    box.style.height = `${Math.min(box.scrollHeight, 150)}px`;
  }

  async function run<T>(action: () => Promise<T>): Promise<T | undefined> {
    try {
      error = "";
      return await action();
    } catch (problem) {
      error = String((problem as Error)?.message ?? problem);
      return undefined;
    }
  }

  function start() {
    live = { text: "", thinking: "", phase: "loading", status: "" };
    stuck = true;
  }

  async function submit() {
    const text = draft.trim();
    if (text === "" || busy || off) return;
    draft = "";
    start();
    const next = await run(() => Chat.Send(text));
    if (!next) {
      live = null;
      draft = text; // not sent: the words are not lost
    } else {
      view = next;
    }
    await tick();
    grow();
    toEnd(true);
  }

  async function ask(text: string) {
    start();
    const next = await run(() => Chat.AskNew(text));
    if (next) view = next;
    else live = null;
    opened = {};
    toEnd(true);
  }

  async function retry() {
    start();
    const next = await run(() => Chat.Regenerate());
    if (next) view = next;
    else live = null;
    toEnd(true);
  }

  async function stop() {
    await Chat.Stop();
  }

  async function newChat() {
    const next = await run(() => Chat.New());
    if (next) view = next;
    live = null;
    opened = {};
    historyOpen = false;
    await tick();
    box?.focus();
  }

  async function openChat(id: string) {
    const next = await run(() => Chat.Open(id));
    if (next) view = next;
    live = null;
    opened = {};
    historyOpen = false;
    toEnd(true);
    await tick();
    box?.focus();
  }

  async function showHistory() {
    historyOpen = !historyOpen;
    confirm = "";
    if (historyOpen) await loadHistory();
  }

  let asked = 0;
  async function loadHistory() {
    const mine = ++asked;
    const found = (await run(() => Chat.List(historyQuery))) ?? [];
    if (mine === asked) history = found;
  }

  async function removeChat(id: string) {
    if (confirm !== id) {
      confirm = id;
      return;
    }
    confirm = "";
    const next = await run(() => Chat.Delete(id));
    if (next) view = next;
    await loadHistory();
  }

  async function clearAll() {
    if (confirm !== "all") {
      confirm = "all";
      return;
    }
    confirm = "";
    const next = await run(() => Chat.ClearHistory());
    if (next) view = next;
    await loadHistory();
  }

  async function togglePin() {
    onTop = !onTop;
    Chat.SetOnTop(onTop);
    try {
      localStorage.setItem(PIN_KEY, String(onTop));
    } catch {
      // a window without storage forgets the choice; it still works
    }
  }

  async function toggleSaved() {
    const next = await run(() => Chat.SetUnsaved(!view.unsaved));
    if (next) view = next;
  }

  async function copyAnswer(text: string) {
    try {
      await Clipboard.SetText(text);
    } catch (problem) {
      error = `Not copied: ${String((problem as Error)?.message ?? problem)}`;
    }
  }

  function onBoxKey(event: KeyboardEvent) {
    if (sends(event, draft)) {
      event.preventDefault();
      submit();
    } else if (event.key === "ArrowUp" && draft === "" && !busy) {
      const last = lastQuestion(messages);
      if (last) {
        event.preventDefault();
        draft = last;
        tick().then(grow);
      }
    }
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.ctrlKey && event.key.toLowerCase() === "n") {
      event.preventDefault();
      newChat();
    } else if (event.ctrlKey && event.key.toLowerCase() === "h") {
      event.preventDefault();
      showHistory();
    } else if (event.key === "Escape") {
      const step = escapeStep(busy, historyOpen, draft);
      if (step === "stop") stop();
      else if (step === "history") historyOpen = false;
      else if (step === "close") Window.Close();
    }
  }

  function onProseClick(event: MouseEvent) {
    const link = (event.target as HTMLElement).closest("a");
    if (link) {
      event.preventDefault(); // a click must not navigate the chat window itself
      Browser.OpenURL(link.getAttribute("href") ?? "");
    }
  }

  // The address says what the window is for: #chat (the open conversation), #chat/new, or #chat/<id>.
  async function follow() {
    const want = location.hash.split("/")[1] ?? "";
    history_replace();
    if (want === "new") await takePending(true);
    else if (want !== "") await openChat(want);
    else {
      const next = await run(() => Chat.Current());
      if (next) view = next;
      await takePending(false);
    }
    toEnd(true);
  }
  const history_replace = () => window.history.replaceState(null, "", "#chat"); // handled: the next request for the same place is heard

  /** A question that came from the bar waits in Go until a window takes it. */
  async function takePending(fresh: boolean) {
    const text = await Chat.TakePending();
    if (text) await ask(text);
    else if (fresh) await newChat();
  }

  onMount(() => {
    document.documentElement.dataset.surface = "settings";
    try {
      onTop = localStorage.getItem(PIN_KEY) !== "false";
    } catch {
      onTop = true;
    }
    Chat.SetOnTop(onTop);
    Chat.Info().then((i) => (info = i));
    follow();

    const offs = [
      Events.On("chat:model", (e) => (info = e.data as Info)),
      Events.On("chat:status", (e) => {
        if (!live) return;
        const s = e.data as { phase: Phase; text: string };
        live.phase = s.phase;
        live.status = s.text ?? "";
      }),
      Events.On("chat:delta", (e) => {
        if (!live) return;
        const d = e.data as { text?: string; thinking?: string };
        live.text += d.text ?? "";
        live.thinking += d.thinking ?? "";
        toEnd();
      }),
      Events.On("chat:done", async (e) => {
        const d = e.data as { view: View; error?: string; restore?: string };
        view = d.view;
        live = null;
        error = d.error ?? "";
        if (d.restore) draft = d.restore;
        await toEnd(true);
        await tick();
        grow();
        box?.focus();
      }),
      Events.On("chat:reset", (e) => {
        view = e.data as View;
        live = null;
        opened = {};
      }),
      Events.On("chat:ask", () => takePending(true)),
    ];
    const clock = setInterval(() => (now = Date.now()), 20_000);
    const refocus = () => box?.focus();
    const leaving = () => void Chat.Stop(); // the window is closing: an answer nobody reads is not worth the graphics card
    window.addEventListener("hashchange", follow);
    window.addEventListener("focus", refocus);
    window.addEventListener("pagehide", leaving);
    return () => {
      offs.forEach((off) => off());
      clearInterval(clock);
      window.removeEventListener("hashchange", follow);
      window.removeEventListener("focus", refocus);
      window.removeEventListener("pagehide", leaving);
    };
  });
</script>

<svelte:window onkeydown={onKeydown} />

<main>
  <header>
    <span class="dot {status.tone}" class:pulse={status.busy} title={status.label} role="img" aria-label={status.label}></span>
    <h1>Omni AI</h1>
    <div class="tools">
      <button
        type="button"
        class="btn quiet"
        class:on={info?.thinking}
        aria-pressed={info?.thinking ?? false}
        disabled={off}
        onclick={() => Chat.SetThinking(!info?.thinking)}
        title="Let the model reason before it answers: slower, sometimes better"
      >
        Think
      </button>
      <button type="button" class="btn quiet" onclick={newChat} title="New chat (Ctrl+N)" aria-label="New chat">New</button>
      <button type="button" class="btn quiet" class:on={historyOpen} aria-pressed={historyOpen} onclick={showHistory} title="Past chats (Ctrl+H)">History</button>
      <button type="button" class="btn quiet" class:on={onTop} aria-pressed={onTop} onclick={togglePin} title="Keep above other windows">On top</button>
    </div>
  </header>

  {#if historyOpen}
    <section class="history" aria-label="Past chats">
      <input
        type="text"
        bind:value={historyQuery}
        oninput={loadHistory}
        placeholder="Search past chats"
        aria-label="Search past chats"
        spellcheck="false"
        autocomplete="off"
      />
      {#if history.length > 0}
        <ul>
          {#each history as chat (chat.id)}
            <li class:open={chat.id === view.id}>
              <button type="button" class="pick" onclick={() => openChat(chat.id)}>
                <span class="title">{chat.title}</span>
                <span class="when">{when(chat.modified)}</span>
              </button>
              <button type="button" class="btn quiet gone" class:danger={confirm === chat.id} onclick={() => removeChat(chat.id)} aria-label="Delete {chat.title}">
                {confirm === chat.id ? "Delete?" : "Delete"}
              </button>
            </li>
          {/each}
        </ul>
        <div class="clear">
          <button type="button" class="btn quiet" class:danger={confirm === "all"} onclick={clearAll}>{confirm === "all" ? "Clear all past chats?" : "Clear history"}</button>
          {#if confirm}<button type="button" class="btn quiet" onclick={() => (confirm = "")}>Keep</button>{/if}
        </div>
      {:else}
        <p class="none">{historyQuery ? `No chat has “${historyQuery}” in it.` : "No past chats yet."}</p>
      {/if}
    </section>
  {/if}

  {#if off}
    <div class="off" role="status">
      <h2>Omni AI is off</h2>
      <p>A small language model that runs on this computer. Turning it on downloads about 1 GB, once; nothing you ask leaves the computer.</p>
      <button type="button" class="btn primary" onclick={() => Chat.OpenSettings()}>Open Settings</button>
    </div>
  {:else}
    <div class="log" bind:this={log} onscroll={onScroll} role="log" aria-live="polite" aria-label="Conversation">
      {#if messages.length === 0 && !live}
        <div class="empty">
          <p class="hello">Ask anything.</p>
          <p class="note">It runs on this computer. It is a small model: check anything that matters.</p>
        </div>
      {/if}
      {#each messages as message, i (i)}
        {#if i === view.memory_start && i > 0}
          <div class="memory" role="separator"><span>Earlier messages are no longer in the model's memory</span></div>
        {/if}
        {#if message.role === "user"}
          <div class="row user"><div class="bubble">{message.text}</div></div>
        {:else}
          {@const html = render(message.text)}
          <div class="row ai">
            {#if message.thinking}
              <button type="button" class="thought" aria-expanded={!!opened[i]} onclick={() => (opened[i] = !opened[i])}>
                <span class="chev" class:down={opened[i]} aria-hidden="true">›</span>{thoughtFor(message.thinking_ms ?? 0)}
              </button>
              {#if opened[i]}<div class="thinking">{message.thinking}</div>{/if}
            {/if}
            <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
            <div class="prose" use:copyable={html} onclick={onProseClick}>{@html html}</div>
            <div class="actions">
              {#if message.stopped}<span class="tag">Stopped</span>{/if}
              <button type="button" class="btn quiet" onclick={() => copyAnswer(message.text)}>Copy</button>
              {#if i === lastAnswer && canRetry}<button type="button" class="btn quiet" onclick={retry}>Try again</button>{/if}
            </div>
          </div>
        {/if}
      {/each}
      {#if live}
        {@const html = render(live.text)}
        <div class="row ai">
          {#if live.thinking && !live.text}<div class="thinking live">{live.thinking}</div>{/if}
          {#if live.text}
            <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
            <div class="prose" use:copyable={html} onclick={onProseClick}>{@html html}</div>
          {:else}
            <p class="waiting"><span class="pip" aria-hidden="true"></span>{waiting(live.phase, live.status)}</p>
          {/if}
        </div>
      {:else if error}
        <div class="row ai">
          <p class="error">{error}</p>
          {#if canRetry}<div class="actions"><button type="button" class="btn quiet" onclick={retry}>Try again</button></div>{/if}
        </div>
      {/if}
    </div>
  {/if}

  <form class="composer" onsubmit={(event) => { event.preventDefault(); submit(); }}>
    <textarea
      bind:this={box}
      bind:value={draft}
      oninput={grow}
      onkeydown={onBoxKey}
      rows="1"
      placeholder={off ? "Turn Omni AI on to ask" : "Ask anything…"}
      disabled={off}
      spellcheck="true"
      aria-label="Message"
    ></textarea>
    {#if busy}
      <button type="button" class="send" onclick={stop} aria-label="Stop (Esc)" title="Stop (Esc)">
        <svg class="line-icon" width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><rect x="6" y="6" width="12" height="12" rx="2" /></svg>
      </button>
    {:else}
      <button type="submit" class="send" disabled={off || draft.trim() === ""} aria-label="Send (Enter)" title="Send (Enter)">
        <svg class="line-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 19V6 M6.5 11.5 12 6l5.5 5.5" /></svg>
      </button>
    {/if}
  </form>
  <footer>
    <button
      type="button"
      class="saved"
      class:off={view.unsaved}
      disabled={off || (info !== null && !info.keep_history)}
      aria-pressed={view.unsaved}
      onclick={toggleSaved}
      title={info && !info.keep_history ? "History is off in Settings" : view.unsaved ? "Save this chat" : "Do not save this chat"}
    >
      {view.unsaved ? "Not saved" : "Saved"}
    </button>
    <span class="meter" title="How much of the model's memory this chat uses">{meter(view.used, view.room)}{unload ? ` · ${unload}` : ""}</span>
  </footer>
</main>

<style>
  main {
    position: relative;
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
    gap: 9px;
    padding: 9px 10px 9px 16px;
    border-bottom: 1px solid var(--line);
  }
  h1 {
    margin: 0;
    font: 620 14px/1 var(--sans);
    letter-spacing: -0.005em;
  }
  .tools {
    display: flex;
    gap: 2px;
    margin-left: auto;
  }
  :global(main .tools .btn) {
    height: 30px;
    padding: 0 10px;
    font-size: 12.5px;
  }
  :global(main .btn.quiet.on) {
    color: var(--text);
    background: var(--lift);
  }
  .dot {
    flex: none;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--text-3);
  }
  .dot.ready {
    background: var(--chrome);
    box-shadow: 0 0 0 3px rgba(230, 232, 236, 0.15);
  }
  .dot.busy {
    background: var(--text-2);
  }
  .dot.bad {
    background: var(--danger);
  }
  .dot.pulse {
    animation: pulse 1.1s ease-in-out infinite;
  }
  @keyframes pulse {
    50% {
      opacity: 0.35;
    }
  }

  .log {
    flex: 1;
    min-height: 0;
    padding: 18px 16px 8px;
    overflow-y: auto;
    user-select: text;
    cursor: auto;
  }
  .empty {
    display: grid;
    place-content: center;
    gap: 6px;
    height: 100%;
    padding-bottom: 40px;
    text-align: center;
  }
  .hello {
    margin: 0;
    font: 560 18px/1.3 var(--display);
    letter-spacing: -0.01em;
  }
  .note {
    margin: 0;
    max-width: 30ch;
    color: var(--text-3);
    font-size: 12.5px;
    line-height: 1.5;
  }
  .row {
    display: flex;
    flex-direction: column;
    margin-bottom: 18px;
  }
  .row.user {
    align-items: flex-end;
  }
  .bubble {
    max-width: 88%;
    padding: 9px 13px;
    border-radius: 16px 16px 5px 16px;
    background: var(--lift);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    line-height: 1.5;
  }
  .row.ai {
    align-items: stretch;
  }
  .prose {
    line-height: 1.58;
    overflow-wrap: anywhere;
  }
  .prose :global(p),
  .prose :global(ul),
  .prose :global(ol),
  .prose :global(blockquote),
  .prose :global(pre) {
    margin: 0 0 10px;
  }
  .prose :global(> :last-child) {
    margin-bottom: 0;
  }
  .prose :global(h1),
  .prose :global(h2),
  .prose :global(h3),
  .prose :global(h4) {
    margin: 14px 0 6px;
    font: 620 14.5px/1.3 var(--sans);
  }
  .prose :global(ul),
  .prose :global(ol) {
    padding-left: 22px;
  }
  .prose :global(li) {
    margin: 2px 0;
  }
  .prose :global(code) {
    padding: 1px 5px;
    border-radius: 5px;
    background: var(--tile);
    font: 450 12.5px/1.4 var(--mono);
  }
  .prose :global(pre) {
    position: relative;
    padding: 12px 14px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--tile);
    overflow-x: auto;
  }
  .prose :global(pre code) {
    padding: 0;
    background: none;
    font-size: 12.5px;
    line-height: 1.55;
    white-space: pre;
  }
  .prose :global(pre .copy) {
    position: absolute;
    top: 6px;
    right: 6px;
    height: 24px;
    padding: 0 8px;
    border: 0;
    border-radius: 7px;
    background: var(--lift);
    color: var(--text-2);
    font: 560 11.5px/1 var(--sans);
    cursor: pointer;
    opacity: 0;
    transition: opacity 0.12s;
  }
  .prose :global(pre:hover .copy),
  .prose :global(pre .copy:focus-visible) {
    opacity: 1;
  }
  .prose :global(blockquote) {
    padding-left: 12px;
    border-left: 2px solid var(--line);
    color: var(--text-2);
  }
  .prose :global(a) {
    color: var(--text);
    text-decoration: underline;
    text-decoration-color: var(--text-3);
    text-underline-offset: 2px;
    cursor: pointer;
  }
  .prose :global(hr) {
    border: 0;
    border-top: 1px solid var(--line);
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 2px;
    margin: 4px 0 0 -10px;
    opacity: 0;
    transition: opacity 0.12s;
  }
  .row.ai:hover .actions,
  .row.ai:focus-within .actions {
    opacity: 1;
  }
  :global(main .actions .btn) {
    height: 26px;
    padding: 0 9px;
    font-size: 12px;
  }
  .tag {
    margin-left: 10px;
    color: var(--text-3);
    font-size: 12px;
  }
  .thought {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    width: fit-content;
    margin: 0 0 6px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--text-3);
    font: 500 12.5px/1.2 var(--sans);
    cursor: pointer;
  }
  .thought:hover {
    color: var(--text-2);
  }
  .chev {
    display: inline-block;
    transition: transform 0.12s;
  }
  .chev.down {
    transform: rotate(90deg);
  }
  .thinking {
    margin: 0 0 10px;
    padding-left: 12px;
    border-left: 2px solid var(--line);
    color: var(--text-3);
    font-size: 12.5px;
    line-height: 1.55;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .thinking.live {
    max-height: 7.5em;
    overflow: hidden;
  }
  .waiting {
    display: flex;
    align-items: center;
    gap: 9px;
    margin: 0;
    color: var(--text-3);
    font-size: 13px;
  }
  .pip {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--text-2);
    animation: pulse 1.1s ease-in-out infinite;
  }
  .error {
    margin: 0;
    color: var(--danger);
    font-size: 13px;
    line-height: 1.5;
  }
  .memory {
    display: flex;
    align-items: center;
    gap: 10px;
    margin: 4px 0 18px;
    color: var(--text-3);
    font-size: 11.5px;
  }
  .memory::before,
  .memory::after {
    content: "";
    flex: 1;
    height: 1px;
    background: var(--line);
  }

  .off {
    flex: 1;
    display: grid;
    place-content: center;
    justify-items: center;
    gap: 10px;
    padding: 24px 32px 48px;
    text-align: center;
  }
  .off h2 {
    margin: 0;
    font: 620 17px/1.3 var(--sans);
  }
  .off p {
    margin: 0 0 6px;
    max-width: 34ch;
    color: var(--text-2);
    font-size: 13px;
    line-height: 1.55;
  }

  .composer {
    display: flex;
    align-items: flex-end;
    gap: 8px;
    margin: 6px 12px 0;
    padding: 6px 6px 6px 14px;
    border: 1px solid var(--line);
    border-radius: 18px;
    background: var(--tile);
    transition: border-color 0.12s;
  }
  .composer:focus-within {
    border-color: rgba(230, 232, 236, 0.45);
  }
  textarea {
    flex: 1;
    min-width: 0;
    max-height: 150px;
    padding: 7px 0;
    border: 0;
    outline: 0;
    resize: none;
    background: transparent;
    color: var(--text);
    font: 450 14px/1.45 var(--sans);
  }
  textarea::placeholder {
    color: var(--text-3);
  }
  .send {
    flex: none;
    display: grid;
    place-items: center;
    width: 32px;
    height: 32px;
    border: 0;
    border-radius: 50%;
    background: var(--chrome);
    color: #000;
    cursor: pointer;
    transition: background-color 0.12s;
  }
  .send:hover:not(:disabled) {
    background: #fff;
  }
  .send:disabled {
    background: var(--lift);
    color: var(--text-3);
    cursor: default;
  }
  .send:focus-visible,
  .saved:focus-visible,
  .pick:focus-visible,
  .thought:focus-visible {
    outline: 1px solid var(--chrome);
    outline-offset: 2px;
  }
  footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 16px 8px;
    color: var(--text-3);
    font-size: 11.5px;
    font-variant-numeric: tabular-nums;
  }
  .saved {
    padding: 2px 0;
    border: 0;
    background: none;
    color: var(--text-3);
    font: inherit;
    cursor: pointer;
  }
  .saved:hover:not(:disabled) {
    color: var(--text-2);
  }
  .saved.off {
    color: var(--text-2);
  }
  .saved:disabled {
    cursor: default;
  }

  .history {
    position: absolute;
    inset: 49px 0 0;
    z-index: 2;
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 10px 10px 8px;
    background: var(--plate);
  }
  .history ul {
    flex: 1;
    min-height: 0;
    margin: 0;
    padding: 0;
    overflow-y: auto;
    list-style: none;
  }
  .history li {
    display: flex;
    align-items: center;
    border-radius: 10px;
  }
  .history li:hover,
  .history li.open {
    background: var(--tile);
  }
  .pick {
    flex: 1;
    display: flex;
    align-items: baseline;
    gap: 12px;
    min-width: 0;
    padding: 9px 10px;
    border: 0;
    background: none;
    color: var(--text);
    font: 450 13.5px/1.3 var(--sans);
    text-align: left;
    cursor: pointer;
  }
  .title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .when {
    flex: none;
    color: var(--text-3);
    font-size: 12px;
  }
  .gone {
    flex: none;
    height: 28px;
    padding: 0 8px;
    font-size: 12px;
    opacity: 0;
  }
  .history li:hover .gone,
  .history li:focus-within .gone {
    opacity: 1;
  }
  .clear {
    display: flex;
    gap: 4px;
    padding-top: 6px;
    border-top: 1px solid var(--line);
  }
  .none {
    margin: 12px 8px;
    color: var(--text-3);
    font-size: 13px;
  }

  @media (prefers-reduced-motion: reduce) {
    .dot.pulse,
    .pip {
      animation: none;
    }
    .chev {
      transition: none;
    }
  }
</style>
