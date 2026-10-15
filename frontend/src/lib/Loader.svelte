<script lang="ts">
  interface Props {
    /** What is being waited for, in a word or two ("Warming up"). */
    text: string;
  }
  let { text }: Props = $props();
</script>

<!-- A chrome orb that turns, and the words under way: each letter breathes a beat after the last. -->
<span class="loader" role="status" aria-live="polite" aria-label={text}>
  <span class="orb" aria-hidden="true"></span>
  <span class="words" aria-hidden="true">
    {#each [...text] as letter, index}
      <span style:animation-delay="{index * 0.09}s">{letter === " " ? " " : letter}</span>
    {/each}
  </span>
</span>

<style>
  .loader {
    display: inline-flex;
    align-items: center;
    gap: 9px;
    color: var(--text-2);
    font-size: 12.5px;
    font-weight: 500;
    letter-spacing: 0.01em;
    white-space: nowrap;
  }
  .words span {
    display: inline-block;
    animation: breathe 1.8s ease-in-out infinite;
  }
  .orb {
    flex: none;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: conic-gradient(from 0deg, #3b3e47, #f4f6fb 18%, #4a4d57 38%, #aeb3c0 58%, #3b3e47 78%, #f4f6fb);
    -webkit-mask: radial-gradient(circle, transparent 3.6px, #000 4.2px);
    mask: radial-gradient(circle, transparent 3.6px, #000 4.2px);
    box-shadow: 0 0 10px rgba(223, 228, 238, 0.35);
    animation: turn 1.5s linear infinite;
  }
  @keyframes breathe {
    0%,
    100% {
      opacity: 0.38;
    }
    50% {
      opacity: 1;
    }
  }
  @keyframes turn {
    to {
      transform: rotate(360deg);
    }
  }
</style>
