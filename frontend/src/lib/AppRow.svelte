<script lang="ts">
  import type { Item as Entry } from "../../bindings/agi/internal/palette";
  import Icon from "./Icon.svelte";

  interface Props {
    entry: Entry;
    id: string;
    selected: boolean;
    onhover: () => void;
    onactivate: () => void;
  }
  let { entry, id, selected, onhover, onactivate }: Props = $props();

  // An icon the launcher could not make (the handler answers 404) keeps the plain glyph.
  let iconFailed = $state(false);

  // Chromium sends a synthetic mousemove after every layout change while the pointer rests on an
  // element (results appearing under a parked pointer); only real movement should select.
  function onpointer(event: MouseEvent) {
    if (event.movementX !== 0 || event.movementY !== 0) onhover();
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div {id} class="item" class:selected role="option" aria-selected={selected} tabindex="-1" onmousemove={onpointer} onclick={onactivate}>
  <span class="tile">
    {#if entry.Icon && !iconFailed}
      <img src={entry.Icon} alt="" onerror={() => (iconFailed = true)} />
    {:else}
      <Icon name="app" />
    {/if}
  </span>
  <div class="body">
    <span class="name">{entry.Title}</span>
  </div>
  <span class="kind">{entry.Subtitle}</span>
</div>

<style>
  .item {
    display: flex;
    align-items: center;
    gap: 14px;
    margin: 0 8px;
    padding: 9px 12px;
    border-radius: 16px;
    outline: none;
  }
  .item.selected {
    background: var(--lift);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.05);
  }
  .tile {
    flex: none;
    display: grid;
    place-items: center;
    width: 44px;
    height: 44px;
    border-radius: 13px;
    color: var(--text-2);
    overflow: hidden;
  }
  /* An app's own icon sits on the plate with no tile behind it, a little smaller than the tile. */
  .tile img {
    width: 36px;
    height: 36px;
    object-fit: contain;
  }
  .body {
    flex: 1;
    min-width: 0;
  }
  .name {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text);
    font-size: 14.5px;
    font-weight: 520;
    letter-spacing: -0.005em;
  }
  .kind {
    flex: none;
    color: var(--text-3);
    font: 400 11.5px var(--mono);
  }
</style>
