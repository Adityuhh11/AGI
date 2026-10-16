<script lang="ts">
  interface Props {
    size?: number;
    /** A search is running: the glint circles the lens. */
    busy?: boolean;
  }
  let { size = 20, busy = false }: Props = $props();
</script>

<!-- A hairline lens with a short detached handle and a small glint inside it. The glint is the one
     moving part: it sits still at the top left and circles the lens while a search runs. -->
<svg class="glyph" class:busy width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-linecap="round" aria-hidden="true">
  <circle cx="10.5" cy="10.5" r="6.4" stroke-width="1.3" />
  <path d="M15.7 15.7 19.6 19.6" stroke-width="1.3" />
  <path class="glint" d="M7.5 9.6a3.3 3.3 0 0 1 2.1-2.1" stroke-width="1.1" />
</svg>

<style>
  .glint {
    opacity: 0.75;
    transform-origin: 10.5px 10.5px;
  }
  .busy .glint {
    animation: orbit 1.1s linear infinite;
  }
  @keyframes orbit {
    to {
      transform: rotate(360deg);
    }
  }
</style>
