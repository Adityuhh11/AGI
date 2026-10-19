<script lang="ts">
  import { Events } from "@wailsio/runtime";
  import { capture } from "../accelerator";

  interface Props {
    /** The hotkey as it is now; "" means there is none. */
    value: string;
    /** The id of the element that names this field. */
    labelId: string;
    /** What a Reset button puts back; no button when it is empty. */
    defaultValue?: string;
    /** Whether the hotkey may be turned off (a "Turn off" button). */
    clearable?: boolean;
    /** Saves a new hotkey and returns why it was refused, or "" when it worked. */
    onsave: (accelerator: string) => Promise<string>;
  }
  let { value, labelId, defaultValue = "", clearable = false, onsave }: Props = $props();

  // The field starts from the hotkey as it was when the page opened and then owns what it shows.
  // svelte-ignore state_referenced_locally
  let shown = $state(value);
  let capturing = $state(false);
  let hint = $state("");
  let error = $state("");

  async function commit(accelerator: string) {
    error = await onsave(accelerator);
    if (error === "") shown = accelerator;
  }

  // The page gets no keydown for Alt+Space (Windows takes it as the system-menu key); Go reports it.
  $effect(() =>
    Events.On("hotkey:alt-space", (e) => {
      if (!capturing) return;
      capturing = false;
      hint = "";
      commit(e.data as string);
    }),
  );

  function startCapture() {
    capturing = true;
    hint = "";
  }

  function onKeydown(e: KeyboardEvent) {
    if (!capturing) {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        startCapture();
      }
      return;
    }
    e.preventDefault();
    e.stopPropagation();
    if (e.code === "Escape" && !e.ctrlKey && !e.altKey && !e.shiftKey && !e.metaKey) {
      capturing = false;
      hint = "";
      return;
    }
    const got = capture(e);
    if (got === null) return; // only modifiers so far
    if ("hint" in got) {
      hint = got.hint;
      return;
    }
    capturing = false;
    hint = "";
    commit(got.accelerator);
  }
</script>

<div class="keyrow">
  <button type="button" class="keys" aria-labelledby={labelId} onclick={startCapture} onkeydown={onKeydown} onblur={() => (capturing = false)}>
    {#if capturing}
      <span class="prompt">Press the new combination. Esc cancels.</span>
    {:else if shown === ""}
      <span class="none">Not set</span>
      <span class="change">Set</span>
    {:else}
      {#each shown.split("+") as part (part)}<kbd>{part}</kbd>{/each}
      <span class="change">Change</span>
    {/if}
  </button>
  {#if defaultValue && shown !== defaultValue}
    <button type="button" class="btn quiet" onclick={() => commit(defaultValue)}>Reset to {defaultValue}</button>
  {/if}
  {#if clearable && shown !== ""}
    <button type="button" class="btn quiet" onclick={() => commit("")}>Turn off</button>
  {/if}
</div>
{#if hint}<p class="help">{hint}</p>{/if}
{#if error}<p class="error">{error}</p>{/if}

<style>
  .keyrow {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .keys {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-height: 40px;
    padding: 6px 12px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--tile);
    color: var(--text-2);
    font: 450 13px/1 var(--sans);
    cursor: pointer;
  }
  .keys:hover {
    background: var(--lift);
  }
  .change {
    margin-left: 8px;
    color: var(--text-3);
    font-size: 12px;
  }
  .prompt {
    color: var(--text);
  }
  .none {
    color: var(--text-3);
  }
</style>
