<script lang="ts">
  import type { Snippet } from "svelte";
  import { onMount } from "svelte";

  let { children }: { children: Snippet } = $props();

  // Wails turns the edge of a window with no frame into a resize handle only 5 px wide (3 px at 125%), too thin to hit; the
  // runtime reads these two numbers each time the mouse moves, so a wider handle is a matter of saying so.
  const GRIP = 9;
  onMount(() => {
    const wails = ((window as any)._wails ??= {});
    wails.flags ??= {};
    wails.flags["system.resizeHandleWidth"] = GRIP;
    wails.flags["system.resizeHandleHeight"] = GRIP;
  });

  const EDGES = ["n", "s", "e", "w", "ne", "nw", "se", "sw"] as const;
</script>

<!--
  The look of the notes, the chat and the settings: no frame and no title bar, one rounded card on a clear window with a lit
  edge, like the timer and the search bar. The page decides which part drags it (--wails-draggable: drag) and where its
  close button is; the window stays resizable from its edges. The grips are clear strips around the edge that give the
  resize cursor wherever the pointer is on them: what is under them would otherwise choose the cursor.
-->
<div class="card">{@render children()}</div>
{#each EDGES as edge (edge)}<div class="grip {edge}" aria-hidden="true"></div>{/each}

<style>
  .card {
    position: fixed;
    inset: 0;
    display: flex;
    flex-direction: column;
    border-radius: 26px;
    background: var(--plate);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.16),
      inset 0 0 0 1px rgba(255, 255, 255, 0.1);
    color: var(--text);
  }
  .grip {
    position: fixed;
    z-index: 100;
    --wails-draggable: no-drag;
  }
  .n,
  .s {
    left: 18px;
    right: 18px;
    height: 9px;
    cursor: ns-resize;
  }
  .e,
  .w {
    top: 18px;
    bottom: 18px;
    width: 9px;
    cursor: ew-resize;
  }
  .n {
    top: 0;
  }
  .s {
    bottom: 0;
  }
  .e {
    right: 0;
  }
  .w {
    left: 0;
  }
  .ne,
  .nw,
  .se,
  .sw {
    width: 19px;
    height: 19px;
  }
  .ne,
  .sw {
    cursor: nesw-resize;
  }
  .nw,
  .se {
    cursor: nwse-resize;
  }
  .ne {
    top: 0;
    right: 0;
  }
  .nw {
    top: 0;
    left: 0;
  }
  .se {
    bottom: 0;
    right: 0;
  }
  .sw {
    bottom: 0;
    left: 0;
  }
</style>
