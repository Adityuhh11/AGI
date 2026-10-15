<script lang="ts">
  import type { Match, Row } from "../../bindings/agi/internal/search";
  import { moreCount } from "./flatten";

  interface Props {
    /** The file, or the file a further match belongs to. */
    row: Row;
    /** Set for a further match shown under an expanded file; unset for the file's own line. */
    match?: Match;
    selected: boolean;
    expanded?: boolean;
    onhover: () => void;
    onactivate: () => void;
  }

  let { row, match, selected, expanded = false, onhover, onactivate }: Props = $props();

  const glyphs: Record<string, string> = { code: "{ }", text: "Aa", pdf: "PDF", image: "IMG" };
  const shown = $derived(match ?? row.Matches?.[0]);
  const more = $derived(moreCount(row));

  // Chromium sends a synthetic mousemove after every layout change while the pointer rests on an
  // element (results appearing under a parked pointer); only real movement should select.
  function onpointer(event: MouseEvent) {
    if (event.movementX !== 0 || event.movementY !== 0) onhover();
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="item" class:selected class:child={match} onmousemove={onpointer} onclick={onactivate}>
  {#if match}
    <span class="indent"></span>
  {:else}
    <span class="glyph">{glyphs[row.Kind] ?? "·"}</span>
  {/if}
  <div class="body">
    {#if !match}
      <div class="title">
        <span class="name">{row.Name}</span>
        <span class="folder">{row.Folder}</span>
      </div>
    {/if}
    {#if shown && (shown.Location || shown.Snippet) && row.Kind !== "image"}
      <div class="detail">
        {#if shown.Location}<span class="where">{shown.Location}</span>{/if}
        <span class="snippet">{shown.Snippet}</span>
      </div>
    {/if}
  </div>
  {#if !match && more > 0}
    <span class="more">{expanded ? "−" : "+"}{more} more</span>
  {/if}
</div>

<style>
  .item {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 9px 24px;
    cursor: default;
  }
  .item.selected {
    background: #181818;
  }
  .glyph {
    flex: none;
    width: 34px;
    padding-top: 2px;
    color: #777;
    font: 600 11px Consolas, monospace;
    text-align: center;
  }
  .indent {
    flex: none;
    width: 34px;
  }
  .body {
    flex: 1;
    min-width: 0;
  }
  .title {
    display: flex;
    gap: 10px;
    align-items: baseline;
    white-space: nowrap;
  }
  .name {
    color: #f2f2f2;
    font-size: 15px;
  }
  .folder {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    direction: rtl; /* keep the end of a long path visible */
    color: #666;
    font-size: 12px;
  }
  .detail {
    margin-top: 3px;
    color: #9a9a9a;
    font-size: 12.5px;
    line-height: 1.4;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
  }
  .where {
    margin-right: 8px;
    color: #6f8fb5;
    font-family: Consolas, monospace;
    font-size: 11.5px;
  }
  .more {
    flex: none;
    color: #666;
    font-size: 11.5px;
    padding-top: 3px;
  }
  .child .detail {
    margin-top: 0;
  }
</style>
