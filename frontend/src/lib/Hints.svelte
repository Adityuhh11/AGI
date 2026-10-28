<script lang="ts">
  import type { Info } from "../../bindings/omni/internal/status";
  import { available, pastes } from "./actions";
  import { moment } from "./format";
  import { timeOf, type Item } from "./flatten";

  interface Props {
    /** The selected row, if any: the hints name what its keys do. */
    current: Item | undefined;
    /** The program a moment in a recording opens in ("VLC"), or empty when there is none. */
    player: string;
    /** Whether the selected row is pinned; null if it cannot be pinned. */
    pinned: boolean | null;
    info: Info | null;
  }
  let { current, player, pinned, info }: Props = $props();
  const at = $derived(current && current.kind !== "entry" ? timeOf(current) : null);
</script>

<footer class="hints">
  <span><kbd>Enter</kbd>{at !== null && player ? `play at ${moment(at)} in ${player}` : current && pastes(current) ? "paste" : "open"}</span>
  {#if current && available(current, "reveal")}<span><kbd>Ctrl+Enter</kbd>reveal</span>{/if}
  {#if current && available(current, "copy")}<span><kbd>Ctrl+Enter</kbd>copy</span>{/if}
  <span><kbd>Ctrl+K</kbd>actions</span>
  {#if pinned !== null}<span><kbd>Ctrl+P</kbd>{pinned ? "unpin" : "pin"}</span>{/if}
  {#if current && current.kind !== "entry" && current.row.Matches && current.row.Matches.length > 1}
    <span><kbd>→</kbd>more</span>
  {/if}
  <span><kbd>Tab</kbd>mode</span>
  <span><kbd>Alt+Enter</kbd>web</span>
  <span><kbd>Esc</kbd>close</span>
  {#if info && info.IndexText && info.IndexState !== "uptodate"}
    <span class="index">{info.IndexText}</span>
  {/if}
</footer>

<style>
  .hints {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 4px 16px;
    padding: 11px 24px 13px;
    border-top: 1px solid var(--line);
    color: var(--text-3);
    font-size: 11.5px;
  }
  kbd {
    margin-right: 7px;
    padding: 2px 7px;
    border-radius: 7px;
    background: var(--tile);
    color: var(--text-2);
    font: 500 10.5px var(--mono);
  }
  .index {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    text-align: right;
    font-family: var(--mono);
    font-size: 11px;
  }
</style>
