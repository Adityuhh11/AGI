<script lang="ts">
  import Icon from "./Icon.svelte";

  interface Props {
    icon: "plus" | "history" | "close" | "brain" | "send" | "stop";
    /** The name a screen reader says; also the tooltip unless `title` is given. */
    label: string;
    title?: string;
    /** `quiet` sits in a header with no fill; `solid` is a filled circle (send). */
    look?: "quiet" | "tile" | "solid";
    /** A switch that is on: the button is lit. */
    on?: boolean;
    /** Set for a switch, so a screen reader says whether it is on. */
    pressed?: boolean;
    disabled?: boolean;
    size?: number;
    type?: "button" | "submit";
    onclick?: (event: MouseEvent) => void;
  }
  let { icon, label, title, look = "quiet", on = false, pressed, disabled = false, size = 34, type = "button", onclick }: Props = $props();
</script>

<button {type} class="round {look}" class:on aria-label={label} aria-pressed={pressed} title={title ?? label} {disabled} {onclick} style:width="{size}px" style:height="{size}px">
  <Icon name={icon} size={Math.round(size * 0.5)} />
</button>

<style>
  .round {
    flex: none;
    display: grid;
    place-items: center;
    padding: 0;
    border: 0;
    border-radius: 50%;
    cursor: pointer;
    transition: background-color 0.12s, color 0.12s;
    --wails-draggable: no-drag;
  }
  .quiet {
    background: transparent;
    color: var(--text-2);
  }
  .quiet:hover:not(:disabled),
  .quiet.on {
    background: var(--lift);
    color: var(--text);
  }
  .tile {
    background: var(--lift);
    color: var(--text-2);
  }
  .tile:hover:not(:disabled) {
    color: var(--text);
    background: rgba(255, 255, 255, 0.13);
  }
  /* a switch that is on is lit like the search card's metal: bright fill, black glyph */
  .tile.on {
    background: var(--chrome);
    color: #000;
  }
  .tile.on:hover:not(:disabled) {
    background: #fff;
    color: #000;
  }
  .solid {
    background: var(--chrome);
    color: #000;
  }
  .solid:hover:not(:disabled) {
    background: #fff;
  }
  .round:disabled {
    background: var(--lift);
    color: var(--text-3);
    cursor: default;
  }
  .round:focus-visible {
    outline: 1px solid var(--chrome);
    outline-offset: 2px;
  }
</style>
