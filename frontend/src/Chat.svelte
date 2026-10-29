<script lang="ts">
  import { onMount, tick } from "svelte";
  import { Browser, Clipboard, Events, Window } from "@wailsio/runtime";
  import { Service as Chat } from "../bindings/omni/internal/chat";
  import type { Info, Summary, View } from "../bindings/omni/internal/chat";
  import "./lib/settings/forms.css";
  import Card from "./lib/Card.svelte";
  import LiquidRing from "./lib/LiquidRing.svelte";
  import RoundButton from "./lib/RoundButton.svelte";
  import Icon from "./lib/Icon.svelte";
  import PlusMenu from "./lib/chat/PlusMenu.svelte";
  import { METALS } from "./lib/palettes";
  import { dot, escapeStep, lastQuestion, meter, sends, thoughtFor, unloadsIn, waiting, type Phase } from "./lib/chat/chat";
  import { copyable } from "./lib/chat/copy";
  import { when } from "./lib/format";
  import { render } from "./lib/markdown";

  const EMPTY: View = { id: "", title: "", messages: [], memory_start: 0, used: 0, room: 8192, unsaved: false, busy: false };

  let view = $state<View>(EMPTY);
  let info = $state<Info | null>(null);
  let draft = $state("");
  let error = $state("");
  // The answer being written: it is shown from the events, and replaced by the finished message when the answer ends.
  let live = $state<{ text: string; thinking: string; phase: Phase; status: string } | null>(null);
  let focused = $state(false); // the composer has the keyboard: its ring moves
  let historyOpen = $state(false);
  let menuOpen = $state(false); // the + menu above the composer
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
      if (menuOpen) {
        menuOpen = false;
        box?.focus();
        return;
      }
      const step = escapeStep(busy, historyOpen, draft);
      if (step === "stop") stop();
      else if (step === "history") historyOpen = false;
      else if (step === "close") Window.Close();
    }
  }

  const toggleThinking = () => run(() => Chat.SetThinking(!info?.thinking));
  const toggleWeb = () => run(() => Chat.SetWebSearch(!info?.web_search));

  function openSource(url: string) {
    Browser.OpenURL(url);
  }
  const hostOf = (url: string) => {
    try {
      return new URL(url).hostname.replace(/^www\./, "");
    } catch {
      return url;
    }
  };

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
    document.documentElement.dataset.surface = "card";
    Chat.Info().then((i) => (info = i));
    // The box has the keyboard as soon as the window is up: a window that is already in front gets no focus event to do it.
    follow().then(() => tick().then(() => box?.focus()));

    const offs = [
      Events.On("chat:model", (e) => {
        info = e.data as Info;
        now = Date.now(); // the countdown to the unload is read against this
      }),
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

<Card>
  <main>
    <header>
      <span class="dot tone-{status.tone}" class:pulse={status.busy} title={status.label} role="img" aria-label={status.label}></span>
      <h1>Omni AI</h1>
      <div class="tools">
        <RoundButton icon="plus" label="New chat" title="New chat (Ctrl+N)" onclick={newChat} />
        <RoundButton icon="history" label="Past chats" title="Past chats (Ctrl+H)" on={historyOpen} pressed={historyOpen} onclick={showHistory} />
        <RoundButton icon="close" label="Close" title="Close (Esc)" onclick={() => Window.Close()} />
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
      <div class="unavailable" role="status">
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
              {#if message.text}
                <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
                <div class="prose" use:copyable={html} onclick={onProseClick}>{@html html}</div>
              {/if}
              {#if message.stopped}<p class="stopped">Stopped</p>{/if}
              {#if message.sources?.length}
                <div class="sources" aria-label="Sources">
                  {#each message.sources as source (source.url)}
                    <button type="button" class="source" title="{source.title}: {source.url}" onclick={() => openSource(source.url)}>
                      <Icon name="web" size={12} />{hostOf(source.url)}
                    </button>
                  {/each}
                </div>
              {/if}
              <div class="actions">
                {#if message.text}<button type="button" class="btn quiet" onclick={() => copyAnswer(message.text)}>Copy</button>{/if}
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
            {/if}
            <!-- A model that wrote a few words before it went to the web still shows where it is. -->
            {#if !live.text || live.phase === "web"}
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

    <div class="fade-top" aria-hidden="true"></div>
    <div class="fade" aria-hidden="true"></div>
    <div class="dock">
      <!-- The composer floats over the end of the conversation. Its rim is the search card's ring; it moves only while the box
           has the keyboard or an answer is coming, and rests as a still metal edge otherwise. -->
      <form class="composer" onsubmit={(event) => { event.preventDefault(); submit(); }}>
        <LiquidRing speed={busy ? 1.5 : 0.6} running={focused || busy} metal={METALS[0]} radius={27} />
        <div class="plate">
          <div class="line">
            <PlusMenu bind:open={menuOpen} thinking={info?.thinking ?? false} web={info?.web_search ?? false} disabled={off} onthinking={toggleThinking} onweb={toggleWeb} />
            {#if info?.thinking || info?.web_search}
              <!-- What is switched on, beside the + that switched it: a click turns it off. The box keeps its one row. -->
              <div class="pills">
                {#if info?.thinking}
                  <button type="button" class="pill" title="Thinking is on. Click to turn it off" onclick={toggleThinking}>
                    <Icon name="brain" size={14} />Think
                  </button>
                {/if}
                {#if info?.web_search}
                  <button type="button" class="pill" title="Web search is on. Click to turn it off" onclick={toggleWeb}>
                    <Icon name="web" size={14} />Web
                  </button>
                {/if}
              </div>
            {/if}
            <textarea
              bind:this={box}
              bind:value={draft}
              oninput={grow}
              onkeydown={onBoxKey}
              onfocus={() => (focused = true)}
              onblur={() => (focused = false)}
              rows="1"
              placeholder={off ? "Turn Omni AI on to ask" : "Ask anything…"}
              disabled={off}
              spellcheck="true"
              aria-label="Message"
            ></textarea>
            {#if busy}
              <RoundButton icon="stop" look="solid" label="Stop (Esc)" onclick={stop} />
            {:else}
              <RoundButton icon="send" look="solid" type="submit" label="Send (Enter)" disabled={off || draft.trim() === ""} />
            {/if}
          </div>
        </div>
      </form>
      <div class="under">
        <button
          type="button"
          class="saved"
          class:unsaved={view.unsaved}
          disabled={off || (info !== null && !info.keep_history)}
          aria-pressed={view.unsaved}
          onclick={toggleSaved}
          title={info && !info.keep_history ? "History is off in Settings" : view.unsaved ? "Save this chat" : "Do not save this chat"}
        >
          {view.unsaved ? "Not saved" : "Saved"}
        </button>
        <span class="meter" title="How much of the model's memory this chat uses">{meter(view.used, view.room)}{unload ? ` · ${unload}` : ""}</span>
      </div>
    </div>
  </main>
</Card>

<style>
  main {
    position: relative;
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
    max-width: none;
    color: var(--text);
  }
  header {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 14px 8px 24px;
    --wails-draggable: drag; /* the window is moved by holding the header anywhere but on a button */
    user-select: none;
  }
  h1 {
    margin: 0;
    font: 620 14px/1 var(--sans);
    letter-spacing: -0.005em;
  }
  .tools {
    display: flex;
    gap: 4px;
    margin-left: auto;
  }
  .dot {
    flex: none;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--text-3);
  }
  .dot.tone-ready {
    background: var(--chrome);
    box-shadow: 0 0 0 3px rgba(230, 232, 236, 0.15);
  }
  .dot.tone-busy {
    background: var(--text-2);
  }
  .dot.tone-bad {
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
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
    padding: 12px max(24px, calc((100% - 640px) / 2)) 170px;
    overflow-y: auto;
    user-select: text;
    cursor: auto;
  }
  .empty {
    display: grid;
    flex: 1;
    place-content: center;
    gap: 6px;
    padding-bottom: 60px;
    text-align: center;
  }
  .hello {
    margin: 0;
    font: 560 20px/1.3 var(--display);
    letter-spacing: -0.01em;
  }
  .note {
    margin: 0;
    color: var(--text-3);
    font-size: 12.5px;
    line-height: 1.5;
  }
  .row {
    display: flex;
    flex: none;
    flex-direction: column;
    margin-bottom: 18px;
  }
  .row.user {
    align-items: flex-end;
  }
  .bubble {
    max-width: 88%;
    padding: 9px 14px;
    border-radius: 18px 18px 6px 18px;
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
    border-radius: 12px;
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
    border-radius: 12px;
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
  .stopped {
    margin: 6px 0 0;
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
    flex: none;
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

  .unavailable {
    flex: 1;
    display: grid;
    place-content: center;
    justify-items: center;
    gap: 10px;
    padding: 24px 32px 120px;
    text-align: center;
  }
  .unavailable h2 {
    margin: 0;
    font: 620 17px/1.3 var(--sans);
  }
  .unavailable p {
    margin: 0 0 6px;
    max-width: 38ch;
    color: var(--text-2);
    font-size: 13px;
    line-height: 1.55;
  }

  /* Messages scroll up under the header through a soft edge, not a hard cut. */
  .fade-top {
    position: absolute;
    top: 54px;
    right: 0;
    left: 0;
    z-index: 1;
    height: 22px;
    background: linear-gradient(to bottom, #000, rgba(0, 0, 0, 0));
    pointer-events: none;
  }
  /* The end of the conversation fades under the composer. Its corners follow the card's, or they would paint over the clear window. */
  .fade {
    position: absolute;
    right: 0;
    bottom: 0;
    left: 0;
    height: 150px;
    border-radius: 0 0 26px 26px;
    background: linear-gradient(to bottom, rgba(0, 0, 0, 0), #000 62%);
    pointer-events: none;
  }
  .dock {
    position: absolute;
    right: 0;
    bottom: 10px;
    left: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 5px;
    pointer-events: none;
  }
  .dock > * {
    width: min(580px, calc(100% - 80px));
    pointer-events: auto;
  }
  .composer {
    position: relative;
    padding: 2.5px; /* the metal of the ring around the plate */
    border-radius: 29px;
  }
  .plate {
    position: relative;
    display: flex;
    flex-direction: column;
    padding: 6px;
    border-radius: 26.5px;
    background: #0d0d0e;
  }
  .line {
    display: flex;
    align-items: flex-end;
    gap: 8px;
  }
  .pills {
    flex: none;
    display: flex;
    align-items: center;
    gap: 4px;
    height: 34px; /* the height of the round buttons, so the pills sit level with them when the box grows */
  }
  .pill {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    height: 26px;
    padding: 0 10px 0 8px;
    border: 0;
    border-radius: 13px;
    background: var(--lift);
    color: var(--text-2);
    font: 560 12px/1 var(--sans);
    cursor: pointer;
    --wails-draggable: no-drag;
  }
  .pill:hover {
    color: var(--text);
    background: var(--tile);
  }
  .pill:focus-visible {
    outline: 1px solid var(--chrome);
    outline-offset: 2px;
  }
  .sources {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 8px;
  }
  .source {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    max-width: 100%;
    height: 24px;
    padding: 0 9px;
    border: 1px solid var(--line);
    border-radius: 12px;
    background: none;
    color: var(--text-3);
    font: 500 11.5px/1 var(--sans);
    cursor: pointer;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .source:hover {
    color: var(--text);
    background: var(--tile);
  }
  .source:focus-visible {
    outline: 1px solid var(--chrome);
    outline-offset: 2px;
  }
  textarea {
    flex: 1;
    box-sizing: border-box; /* grow() sets the height to scrollHeight, which already holds the padding */
    min-width: 0;
    max-height: 150px;
    padding: 8px 2px;
    border: 0;
    outline: 0;
    resize: none;
    background: transparent;
    color: var(--text);
    font: 450 14px/1.45 var(--sans);
    user-select: text;
  }
  textarea::placeholder {
    color: var(--text-3);
  }
  .under {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 12px;
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
  .saved.unsaved {
    color: var(--text-2);
  }
  .saved:disabled {
    cursor: default;
  }
  .saved:focus-visible,
  .pick:focus-visible,
  .thought:focus-visible {
    outline: 1px solid var(--chrome);
    outline-offset: 2px;
  }

  .history {
    position: absolute;
    inset: 56px 0 0;
    z-index: 3;
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 6px 18px 14px;
    border-radius: 0 0 26px 26px;
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
    border-radius: 12px;
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
