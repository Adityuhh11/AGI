<script lang="ts">
  import type { Item as Entry } from "../../bindings/agi/internal/palette";
  import Icon from "./Icon.svelte";

  interface Props {
    entry: Entry;
    id: string;
    selected: boolean;
    onhover: () => void;
    onactivate: () => void;
    /** The tags this thing carries. */
    tags?: string[];
    /** Whether it is pinned. */
    pinned?: boolean;
  }
  let { entry, id, selected, onhover, onactivate, tags = [], pinned = false }: Props = $props();

  const glyph = $derived(
    entry.Kind === "app" ? "app" : entry.Kind === "calc" ? "calc" : entry.Kind === "system" ? "system" : entry.Kind === "note" ? "note" : entry.Kind === "timer" ? "timer" : entry.Kind === "tag" ? "tag" : entry.Kind === "file" ? "file" : entry.ID.startsWith("cmd:settings:") ? "settings" : entry.Kind === "clip" || entry.ID === "cmd:clipboard" ? "clip" : entry.Kind === "snippet" || entry.Kind === "cmd" ? "snippet" : "web",
  );

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
  <span class="tile" class:plain={!entry.Icon || iconFailed}>
    {#if entry.Icon && !iconFailed}
      <img src={entry.Icon} alt="" onerror={() => (iconFailed = true)} />
    {:else}
      <Icon name={glyph} />
    {/if}
  </span>
  <div class="body">
    <span class="name">{entry.Title}</span>
  </div>
  {#if pinned}<span class="pin" title="Pinned"><Icon name="pin" size={14} /></span>{/if}
  {#if tags.length > 0}
    <span class="tags">{#each tags as tag}<span class="tag">#{tag}</span>{/each}</span>
  {/if}
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
  /* A row without an icon of its own (the web) gets the same tile files have. */
  .tile.plain {
    background: var(--tile);
  }
  .selected .tile.plain {
    color: var(--chrome);
    background: rgba(255, 255, 255, 0.08);
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
  .pin {
    flex: none;
    align-self: center;
    display: grid;
    color: var(--text-3);
  }
  .tags {
    flex: none;
    display: flex;
    gap: 5px;
    align-self: center;
  }
  .tag {
    padding: 1px 8px;
    border-radius: 999px;
    background: var(--tile);
    color: var(--text-2);
    font: 500 10.5px var(--mono);
  }
  .kind {
    flex: none;
    color: var(--text-3);
    font: 400 11.5px var(--mono);
  }
</style>
