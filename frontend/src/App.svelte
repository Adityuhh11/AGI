<script lang="ts">
  import { onMount } from "svelte";
  import { Service as Status } from "../bindings/agi/internal/status";
  import type { Info } from "../bindings/agi/internal/status";

  let info = $state<Info | null>(null);
  let input: HTMLInputElement;

  onMount(async () => {
    input.focus();
    info = await Status.Info();
  });
</script>

<main>
  <input bind:this={input} type="text" placeholder="Search" spellcheck="false" autocomplete="off" />
  {#if info}
    <p class="hint">{info.Hotkey} · {info.Roots?.length ?? 0} folder(s)</p>
  {/if}
</main>

<style>
  :global(html, body) {
    margin: 0;
    height: 100%;
    background: #000;
    color: #eee;
    font-family: "Segoe UI", system-ui, sans-serif;
  }
  input {
    box-sizing: border-box;
    width: 100%;
    padding: 18px 20px;
    border: 0;
    outline: 0;
    background: #000;
    color: #fff;
    font-size: 20px;
  }
  .hint {
    margin: 0;
    padding: 0 20px;
    color: #666;
    font-size: 12px;
  }
</style>
