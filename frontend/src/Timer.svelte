<script lang="ts">
  import { onMount } from "svelte";
  import { Service as Timer } from "../bindings/omni/internal/timer";
  import type { State } from "../bindings/omni/internal/timer";
  import { clock } from "./lib/format";

  const POLL_MS = 250;

  let state = $state<State | null>(null);

  const progress = $derived(state && state.Total > 0 ? 1 - state.Remaining / state.Total : 0);
  const heading = $derived.by(() => {
    if (!state) return "";
    if (state.Finished) return state.Phase === "countdown" ? "Time’s up" : state.Next ? "Done" : "All done";
    return clock(state.Remaining);
  });
  const caption = $derived.by(() => {
    if (!state) return "";
    const round = state.Rounds > 0 && state.Phase === "focus" ? ` · ${state.Round} of ${state.Rounds}` : "";
    const where = state.Paused ? "Paused · " : "";
    return `${where}${state.Label}${round}`;
  });

  async function refresh() {
    try {
      state = await Timer.State();
    } catch {
      // the program is closing the window; the next look finds nothing to show
    }
  }

  onMount(() => {
    refresh();
    const id = setInterval(refresh, POLL_MS);
    return () => clearInterval(id);
  });
</script>

{#if state?.Active}
  <div class="timer" class:finished={state.Finished} class:paused={state.Paused} role="timer" aria-label="{state.Label}, {heading}">
    <div class="top">
      <span class="time" aria-live="off">{heading}</span>
      <div class="buttons">
        {#if state.Finished}
          {#if state.Next}
            <button type="button" class="next" onclick={() => Timer.Skip()}>Start {state.Next.toLowerCase()}</button>
          {/if}
        {:else}
          <button type="button" onclick={() => (state?.Paused ? Timer.Resume() : Timer.Pause())} aria-label={state.Paused ? "Resume" : "Pause"} title={state.Paused ? "Resume" : "Pause"}>
            {#if state.Paused}
              <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M8 5.5v13l11-6.5z" /></svg>
            {:else}
              <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M7 5h3.5v14H7zM13.5 5H17v14h-3.5z" /></svg>
            {/if}
          </button>
          {#if state.Next}
            <button type="button" onclick={() => Timer.Skip()} aria-label="Skip to {state.Next}" title="Skip to {state.Next.toLowerCase()}">
              <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M6 5.5v13l9-6.5zM16.5 5H19v14h-2.5z" /></svg>
            </button>
          {/if}
        {/if}
        <button type="button" onclick={() => Timer.Stop()} aria-label={state.Finished ? "Dismiss" : "Stop"} title={state.Finished ? "Dismiss" : "Stop"}>
          <svg class="line-icon" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-linecap="round"><path d="M7 7l10 10M17 7L7 17" /></svg>
        </button>
      </div>
    </div>
    <div class="caption">{caption}</div>
    <div class="track" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.round(progress * 100)} aria-label="Progress">
      <div class="fill" style:width="{(state.Finished ? 1 : progress) * 100}%"></div>
    </div>
  </div>
{/if}

<style>
  .timer {
    box-sizing: border-box;
    width: 100vw;
    height: 100vh;
    padding: 12px 14px 12px 18px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    border-radius: 26px;
    background: var(--plate);
    box-shadow:
      inset 0 1px 0 color-mix(in srgb, var(--ink) 16%, transparent),
      inset 0 0 0 1px color-mix(in srgb, var(--ink) 10%, transparent);
    color: var(--text);
    --wails-draggable: drag; /* the window is moved by holding it anywhere but on a button */
    user-select: none;
  }
  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  .time {
    font: 600 27px/1 var(--display);
    letter-spacing: -0.02em;
    font-variant-numeric: tabular-nums;
  }
  .paused .time {
    color: var(--text-2);
  }
  .caption {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-2);
    font: 500 12px/1.2 var(--sans);
  }
  .buttons {
    display: flex;
    align-items: center;
    gap: 2px;
    --wails-draggable: no-drag;
  }
  button {
    display: grid;
    place-items: center;
    min-width: 30px;
    height: 30px;
    padding: 0;
    border: 0;
    border-radius: 999px;
    background: transparent;
    color: var(--text-2);
    cursor: pointer;
    transition: background-color 0.12s, color 0.12s;
    --wails-draggable: no-drag;
  }
  button:hover {
    background: var(--lift);
    color: var(--text);
  }
  button:active {
    background: var(--tile);
  }
  button:focus-visible {
    outline: 1px solid var(--chrome);
    outline-offset: 1px;
  }
  .next {
    padding: 0 12px;
    white-space: nowrap;
    background: var(--chrome);
    color: var(--on-chrome);
    font: 600 12px/1 var(--sans);
  }
  .next:hover {
    background: var(--chrome-hot);
    color: var(--on-chrome);
  }
  .track {
    height: 4px;
    border-radius: 2px;
    background: var(--tile);
    overflow: hidden;
  }
  .fill {
    height: 100%;
    border-radius: 2px;
    background: var(--chrome);
    transition: width 0.4s linear; /* a real width keeps the rounded end sharp; a stretched (scaleX) element smears it */
  }
  /* A finished period pulses its edge until it is dismissed; reduced motion holds it still (theme.css). */
  .finished {
    animation: pulse 1.4s ease-in-out infinite;
  }
  @keyframes pulse {
    50% {
      box-shadow:
        inset 0 1px 0 color-mix(in srgb, var(--ink) 30%, transparent),
        inset 0 0 0 2px color-mix(in srgb, var(--chrome) 75%, transparent);
    }
  }
</style>
