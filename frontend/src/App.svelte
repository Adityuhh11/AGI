<script lang="ts">
  import { onMount } from "svelte";
  import { Service as Panel } from "../bindings/agi/internal/panel";
  import { Service as Status } from "../bindings/agi/internal/status";
  import type { Info } from "../bindings/agi/internal/status";

  let info = $state<Info | null>(null);
  let input: HTMLInputElement;

  // The panel is only cloaked when closed, so the page keeps running: refocus and select the
  // old text every time the window regains the keyboard, so typing replaces it.
  function takeFocus() {
    input.focus();
    input.select();
    Status.Info().then((i) => (info = i));
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === "Escape") {
      event.preventDefault();
      Panel.Hide();
    }
  }

  onMount(takeFocus);
</script>

<svelte:window onfocus={takeFocus} onkeydown={onKeydown} />

<main>
  <input bind:this={input} type="text" placeholder="Search" spellcheck="false" autocomplete="off" />
  {#if info?.HotkeyError}
    <p class="problem">{info.HotkeyError}</p>
  {/if}
</main>

<style>
  :global(html, body) {
    margin: 0;
    height: 100%;
    background: #000;
    color: #eee;
    font-family: "Segoe UI", system-ui, sans-serif;
    overflow: hidden;
  }
  input {
    box-sizing: border-box;
    width: 100%;
    padding: 22px 24px;
    border: 0;
    outline: 0;
    background: #000;
    color: #fff;
    font-size: 22px;
  }
  .problem {
    margin: 0;
    padding: 0 24px 12px;
    color: #c66;
    font-size: 12px;
  }
</style>
