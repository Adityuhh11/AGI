<script lang="ts">
  import type { Match, Row } from "../../bindings/agi/internal/search";
  import { moreCount } from "./flatten";
  import { highlight, tidy } from "./highlight";
  import Icon from "./Icon.svelte";
  import { shortFolder } from "./path";

  interface Props {
    /** The file, or the file a further match belongs to. */
    row: Row;
    /** Set for a further match shown under an expanded file; unset for the file's own line. */
    match?: Match;
    /** What the user typed, so the words that matched can be emphasised. */
    query: string;
    id: string;
    selected: boolean;
    expanded?: boolean;
    onhover: () => void;
    onactivate: () => void;
    /** The tags this file carries. */
    tags?: string[];
    /** Whether the file is pinned. */
    pinned?: boolean;
  }

  let { row, match, query, id, selected, expanded = false, onhover, onactivate, tags = [], pinned = false }: Props = $props();

  const icons = { code: "code", text: "text", image: "image", video: "video" } as const;
  const icon = $derived(icons[row.Kind as keyof typeof icons] ?? "file");
  const shown = $derived(match ?? row.Matches?.[0]);
  const more = $derived(moreCount(row));
  // An image the launcher cannot thumbnail (the handler answers 404) keeps its icon.
  let thumbFailed = $state(false);

  // Chromium sends a synthetic mousemove after every layout change while the pointer rests on an
  // element (results appearing under a parked pointer); only real movement should select.
  function onpointer(event: MouseEvent) {
    if (event.movementX !== 0 || event.movementY !== 0) onhover();
  }
</script>

{#snippet marked(text: string)}
  {#each highlight(text, query) as part}{#if part.hit}<mark>{part.text}</mark>{:else}{part.text}{/if}{/each}
{/snippet}

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
  {id}
  class="item"
  class:selected
  class:child={match}
  role="option"
  aria-selected={selected}
  tabindex="-1"
  onmousemove={onpointer}
  onclick={onactivate}
>
  {#if match}
    <span class="indent"></span>
  {:else}
    <span class="tile">
      {#if row.Kind === "image" && !thumbFailed}
        <img src={"/thumb?path=" + encodeURIComponent(row.Path)} alt="" loading="lazy" onerror={() => (thumbFailed = true)} />
      {:else if row.Kind === "pdf"}
        <span class="pdf">PDF</span>
      {:else}
        <Icon name={icon} />
      {/if}
    </span>
  {/if}
  <div class="body">
    {#if !match}
      <div class="title">
        <span class="name">{@render marked(row.Name)}</span>
        <span class="folder" title={row.Folder}>{shortFolder(row.Folder, 2)}</span>
      </div>
    {/if}
    {#if shown && (shown.Location || shown.Snippet) && row.Kind !== "image"}
      <div class="detail">
        {#if shown.Location}<span class="where">{shown.Location}</span>{/if}
        <span class="snippet">{@render marked(tidy(shown.Snippet))}</span>
      </div>
    {/if}
  </div>
  {#if !match && pinned}<span class="pin" title="Pinned"><Icon name="pin" size={14} /></span>{/if}
  {#if !match && tags.length > 0}
    <span class="tags">{#each tags as tag}<span class="tag">#{tag}</span>{/each}</span>
  {/if}
  {#if !match && more > 0}
    <span class="more">{expanded ? "−" : "+"}{more}</span>
  {/if}
</div>

<style>
  .item {
    display: flex;
    align-items: flex-start;
    gap: 14px;
    margin: 0 8px;
    padding: 10px 12px;
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
    background: var(--tile);
    color: var(--text-2);
    overflow: hidden;
  }
  .selected .tile {
    color: var(--chrome);
    background: rgba(255, 255, 255, 0.08);
  }
  .pdf {
    font: 600 11px/1 var(--mono);
    letter-spacing: 0.06em;
  }
  .tile img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .indent {
    flex: none;
    width: 44px;
  }
  .body {
    flex: 1;
    min-width: 0;
    align-self: center;
  }
  .title {
    display: flex;
    align-items: baseline;
    gap: 10px;
    min-width: 0;
  }
  .name {
    flex: none;
    max-width: 62%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text);
    font-size: 14.5px;
    font-weight: 520;
    letter-spacing: -0.005em;
  }
  .folder {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-3);
    font: 400 11.5px var(--mono);
  }
  .detail {
    margin-top: 4px;
    color: var(--text-2);
    font-size: 12.5px;
    line-height: 1.45;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
  }
  .child .detail {
    margin-top: 0;
  }
  .where {
    margin-right: 8px;
    color: var(--text-3);
    font: 400 11px var(--mono);
  }
  mark {
    background: none;
    color: #fff;
    font-weight: 620;
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
  .more {
    flex: none;
    align-self: center;
    padding: 2px 9px;
    border-radius: 999px;
    background: var(--tile);
    color: var(--text-2);
    font: 500 11px var(--mono);
  }
</style>
