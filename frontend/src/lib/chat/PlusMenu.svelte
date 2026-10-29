<script lang="ts">
  import { tick } from "svelte";
  import Icon from "../Icon.svelte";
  import RoundButton from "../RoundButton.svelte";

  interface Props {
    open: boolean;
    thinking: boolean;
    web: boolean;
    disabled?: boolean;
    onthinking: () => void;
    onweb: () => void;
  }
  let { open = $bindable(), thinking, web, disabled = false, onthinking, onweb }: Props = $props();

  let root = $state<HTMLDivElement | undefined>();
  let first = $state<HTMLButtonElement | undefined>();

  // Opening moves the keyboard into the menu, so Tab and Enter work in it.
  $effect(() => {
    if (open) tick().then(() => first?.focus());
  });

  function outside(event: PointerEvent) {
    if (open && root && !root.contains(event.target as Node)) open = false;
  }
</script>

<svelte:window onpointerdown={outside} />

<div class="wrap" bind:this={root}>
  <RoundButton
    icon="plus"
    look="tile"
    label="Tools"
    title="Think, search the web"
    pressed={open}
    {disabled}
    onclick={() => (open = !open)}
  />
  {#if open}
    <div class="menu" role="menu" aria-label="Tools">
      <button bind:this={first} type="button" role="menuitemcheckbox" aria-checked={thinking} class="item" onclick={onthinking}>
        <span class="glyph"><Icon name="brain" size={18} /></span>
        <span class="words"><span class="name">Think</span><span class="hint">Reason before answering. Slower, sometimes better.</span></span>
        <span class="switch" class:on={thinking} aria-hidden="true"></span>
      </button>
      <button type="button" role="menuitemcheckbox" aria-checked={web} class="item" onclick={onweb}>
        <span class="glyph"><Icon name="web" size={18} /></span>
        <span class="words"><span class="name">Search the web</span><span class="hint">Looks things up online. Your search words go to DuckDuckGo.</span></span>
        <span class="switch" class:on={web} aria-hidden="true"></span>
      </button>
    </div>
  {/if}
</div>

<style>
  .wrap {
    position: relative;
    flex: none;
  }
  .menu {
    position: absolute;
    bottom: calc(100% + 12px);
    left: -2px;
    z-index: 6;
    width: 292px;
    padding: 5px;
    border: 1px solid var(--line);
    border-radius: 18px;
    background: #141416;
    box-shadow: 0 12px 34px rgba(0, 0, 0, 0.6);
  }
  .item {
    display: flex;
    align-items: center;
    gap: 11px;
    width: 100%;
    padding: 9px 10px;
    border: 0;
    border-radius: 13px;
    background: none;
    color: var(--text);
    text-align: left;
    cursor: pointer;
    --wails-draggable: no-drag;
  }
  .item:hover,
  .item:focus-visible {
    background: var(--lift);
    outline: 0;
  }
  .glyph {
    flex: none;
    display: grid;
    place-items: center;
    width: 30px;
    height: 30px;
    border-radius: 50%;
    background: var(--tile);
    color: var(--text-2);
  }
  .words {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .name {
    font: 560 13.5px/1.2 var(--sans);
  }
  .hint {
    color: var(--text-3);
    font: 450 11.5px/1.35 var(--sans);
  }
  .switch {
    flex: none;
    position: relative;
    width: 30px;
    height: 18px;
    border-radius: 9px;
    background: var(--lift);
    transition: background-color 0.12s;
  }
  .switch::after {
    content: "";
    position: absolute;
    top: 3px;
    left: 3px;
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background: var(--text-2);
    transition: transform 0.12s, background-color 0.12s;
  }
  .switch.on {
    background: var(--chrome);
  }
  .switch.on::after {
    background: #000;
    transform: translateX(12px);
  }
  @media (prefers-reduced-motion: reduce) {
    .switch,
    .switch::after {
      transition: none;
    }
  }
</style>
