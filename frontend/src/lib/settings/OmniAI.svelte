<script lang="ts">
  import { onMount } from "svelte";
  import { Browser } from "@wailsio/runtime";
  import { OmniAI } from "../../../bindings/omni/internal/settingsui";
  import type { OmniAIView } from "../../../bindings/omni/internal/settingsui";
  import { unloadsIn } from "../chat/chat";

  let page = $state<OmniAIView | null>(null);
  let error = $state("");
  let confirming = $state<"" | "remove" | "clear">("");

  const refresh = async () => {
    page = await OmniAI.View();
  };

  // While a download runs the page asks every second for its progress; otherwise now and then, for the model's state.
  onMount(() => {
    refresh();
    const timer = setInterval(refresh, 1000);
    return () => clearInterval(timer);
  });

  /** Runs a change, shows why it was refused, and shows the page as it is now. */
  async function run(action: () => Promise<string | void>) {
    error = (await action()) ?? "";
    await refresh();
  }

  const megabytes = (bytes: number) => (bytes / (1024 * 1024)).toFixed(bytes < 10 * 1024 * 1024 ? 1 : 0);
  const percent = $derived(page && page.Total > 0 ? Math.min(100, Math.round((page.Done / page.Total) * 100)) : 0);
  const headline = $derived.by(() => {
    if (!page) return "";
    if (page.Installing) return "Downloading";
    if (!page.Installed) return "Not installed";
    if (page.NeedsUpdate) return "Needs an update";
    return page.Enabled ? "On" : "Off";
  });
  const modelText = $derived.by(() => {
    if (!page) return "";
    switch (page.ModelState) {
      case "ready":
        return `In memory, on ${page.Device === "cpu" ? "the processor" : "the graphics card"}${page.UnloadsAt ? `; ${unloadsIn(page.UnloadsAt)}` : ""}.`;
      case "loading":
        return "Starting the model…";
      case "failed":
        return page.ModelMessage || "The model did not start.";
      default:
        return page.ModelMessage || "Not in memory. It starts when you ask.";
    }
  });
  const loadLabels: Record<string, string> = { when_asked: "When I ask something (the model loads the first time)", at_start: "When Omni starts (answers right away, uses memory all day)" };
  const minutesLabel = (m: number) => (m === 0 ? "Never" : m === 1 ? "1 minute" : m < 60 ? `${m} minutes` : "1 hour");
  const daysLabel = (d: number) => (d === 0 ? "For good" : d === 365 ? "A year" : `${d} days`);
  const contextLabel = (n: number) => `${n.toLocaleString("en-US")} tokens${n === 8192 ? " (usual)" : ""}`;

  async function behaviour(change: { load?: string; minutes?: number; context?: number }) {
    if (!page) return;
    await run(() => OmniAI.SetBehaviour(change.load ?? page!.Load, change.minutes ?? page!.UnloadAfterMinutes, change.context ?? page!.ContextTokens));
  }

  async function history(change: { keep?: boolean; days?: number }) {
    if (!page) return;
    await run(() => OmniAI.SetHistory(change.keep ?? page!.KeepHistory, change.days ?? page!.HistoryDays));
  }

  async function remove() {
    confirming = "";
    await run(() => OmniAI.Remove());
  }

  async function clear() {
    confirming = "";
    await run(() => OmniAI.ClearHistory());
  }
</script>

<h2>Omni AI</h2>
<p class="lede">
  Ask questions in a small chat window. A language model answers on this computer: nothing you write leaves it. Open it by typing
  <span class="mono">ask</span> in the bar.
</p>

{#if page}
  <section class="block">
    <header class="top">
      <span class="dot" class:on={page.Enabled && page.ModelState === "ready"} class:bad={page.ModelState === "failed" || !!page.Error} aria-hidden="true"></span>
      <div class="who">
        <h3>{headline}</h3>
        {#if page.Installed && !page.Installing && !page.NeedsUpdate}<p class="help">{page.Enabled ? modelText : "Installed and off."}</p>{/if}
      </div>
      {#if page.Installing}
        <button type="button" class="btn" onclick={() => run(() => OmniAI.CancelInstall())}>Cancel</button>
      {:else if !page.Installed}
        <button type="button" class="btn primary" onclick={() => run(() => OmniAI.TurnOn())}>Download and turn on</button>
      {:else if page.NeedsUpdate}
        <button type="button" class="btn primary" onclick={() => run(() => OmniAI.Update())}>Update</button>
      {:else if page.Enabled}
        <button type="button" class="btn" onclick={() => run(() => OmniAI.TurnOff())}>Turn off</button>
      {:else}
        <button type="button" class="btn primary" onclick={() => run(() => OmniAI.TurnOn())}>Turn on</button>
      {/if}
    </header>

    {#if page.Installing}
      <div class="bar" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={percent} aria-label={page.StepName}>
        <div class="fill" style:transform="scaleX({(page.Total > 0 ? percent : 4) / 100})"></div>
      </div>
      <p class="help">
        Step {page.Step} of {page.Steps}: {page.StepName}{#if page.Total > 0}, {megabytes(page.Done)} of {megabytes(page.Total)} MB{/if}. You can close this
        window; the download goes on.
      </p>
    {:else if !page.Installed}
      <p class="help">
        A one-time download of about {(page.DownloadMB / 1024).toFixed(1)} GB from {(page.Hosts ?? []).join(" and ")}: the model (Liquid AI's LFM3-1.7B) and the
        program that runs it. Nothing is downloaded until you press the button.
        <button type="button" class="link" onclick={() => Browser.OpenURL(page!.LicenseURL)}>Read the model's licence</button>
      </p>
    {:else if page.NeedsUpdate}
      <p class="help">This version of Omni comes with a newer model server. Updating keeps your chats.</p>
    {:else if page.Enabled && page.ModelState === "ready"}
      <div><button type="button" class="btn" onclick={() => run(() => OmniAI.Unload())}>Unload now</button></div>
    {/if}
    {#if page.Error}<p class="error">The last attempt failed: {page.Error}</p>{/if}
    {#if error}<p class="error">{error}</p>{/if}
  </section>

  <section class="block">
    <h3>When the model is in memory</h3>
    <div class="field">
      <label class="label" for="ai-load">Load the model</label>
      <select id="ai-load" value={page.Load} onchange={(e) => behaviour({ load: e.currentTarget.value })}>
        {#each page.Loads ?? [] as choice}<option value={choice}>{loadLabels[choice] ?? choice}</option>{/each}
      </select>
    </div>
    <div class="field">
      <label class="label" for="ai-unload">Unload it after</label>
      <select id="ai-unload" value={String(page.UnloadAfterMinutes)} onchange={(e) => behaviour({ minutes: Number(e.currentTarget.value) })}>
        {#each page.UnloadChoices ?? [] as minutes}<option value={String(minutes)}>{minutesLabel(minutes)}</option>{/each}
      </select>
      <p class="help">Counted from the end of the last answer. Unloading frees the graphics memory (about 1.2 GB); the next question starts the model again, which takes a few seconds.</p>
    </div>
    <div class="field">
      <label class="label" for="ai-context">How much it can read at once</label>
      <select id="ai-context" value={String(page.ContextTokens)} onchange={(e) => behaviour({ context: Number(e.currentTarget.value) })}>
        {#each page.ContextChoices ?? [] as tokens}<option value={String(tokens)}>{contextLabel(tokens)}</option>{/each}
      </select>
      <p class="help">A conversation longer than this forgets its oldest messages (the chat shows where). Bigger costs little memory, but a small model reads long text less well.</p>
    </div>
    <div class="field">
      <label class="check"><input type="checkbox" checked={page.Thinking} onchange={(e) => run(() => OmniAI.SetThinking(e.currentTarget.checked))} /> Let it think before it answers</label>
      <p class="help">Slower, and sometimes better for puzzles and code. The chat window has a Think button for the same switch.</p>
    </div>
    <div class="field">
      <label class="check"><input type="checkbox" checked={page.OnTop} onchange={(e) => run(() => OmniAI.SetOnTop(e.currentTarget.checked))} /> Keep the chat above other windows</label>
      <p class="help">Off, other windows can cover it and it comes back when you open it again.</p>
    </div>
  </section>

  <section class="block">
    <h3>Past chats</h3>
    <div class="field">
      <label class="check"><input type="checkbox" checked={page.KeepHistory} onchange={(e) => history({ keep: e.currentTarget.checked })} /> Keep my chats</label>
      <p class="help">Saved as plain files in <span class="mono">{page.HistoryFolder}</span>, not encrypted. Nothing leaves this computer. A single chat can be marked "Not saved" in its window.</p>
    </div>
    <div class="field">
      <label class="label" for="ai-days">Delete chats I have not opened for</label>
      <select id="ai-days" value={String(page.HistoryDays)} disabled={!page.KeepHistory} onchange={(e) => history({ days: Number(e.currentTarget.value) })}>
        {#each page.HistoryChoices ?? [] as days}<option value={String(days)}>{daysLabel(days)}</option>{/each}
      </select>
    </div>
    <div class="row">
      <span class="help">{page.Conversations === 1 ? "1 chat" : `${page.Conversations} chats`} saved.</span>
      {#if confirming === "clear"}
        <button type="button" class="btn danger" onclick={clear}>Delete them all</button>
        <button type="button" class="btn quiet" onclick={() => (confirming = "")}>Keep</button>
      {:else}
        <button type="button" class="btn" disabled={page.Conversations === 0} onclick={() => (confirming = "clear")}>Clear history</button>
        <button type="button" class="btn quiet" onclick={() => run(() => OmniAI.OpenHistoryFolder())}>Show the folder</button>
      {/if}
    </div>
  </section>

  {#if page.Installed && !page.Installing}
    <section class="block remove">
      <div class="row">
        {#if confirming === "remove"}
          <button type="button" class="btn danger" onclick={remove}>Delete the model (about 1 GB)</button>
          <button type="button" class="btn quiet" onclick={() => (confirming = "")}>Keep</button>
        {:else}
          <button type="button" class="btn quiet" onclick={() => (confirming = "remove")}>Remove the model…</button>
        {/if}
      </div>
      <p class="help">Turns Omni AI off and deletes the model and its program. Your chats stay.</p>
    </section>
  {/if}
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
  .block .field {
    margin-bottom: 8px;
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
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--chrome) 15%, transparent);
  }
  .dot.bad {
    background: var(--danger);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .row .help {
    margin-right: auto;
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
  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--text-2);
    font: inherit;
    text-decoration: underline;
    text-decoration-color: var(--text-3);
    text-underline-offset: 2px;
    cursor: pointer;
  }
  .link:hover {
    color: var(--text);
  }
</style>
