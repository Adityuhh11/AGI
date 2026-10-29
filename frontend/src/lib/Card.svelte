<script lang="ts">
  import type { Snippet } from "svelte";

  let { children }: { children: Snippet } = $props();
</script>

<!--
  The look of the notes, the chat and the settings: no frame and no title bar, one rounded card on a clear window with a lit
  edge, like the timer and the search bar. The page decides which part drags it (--wails-draggable: drag) and where its
  close button is; the window stays resizable from its edges.
-->
<div class="card">{@render children()}</div>

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
</style>
