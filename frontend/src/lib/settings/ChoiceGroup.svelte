<script lang="ts">
  import type { Option } from "./choices";

  interface Props {
    /** The id of the element that names the group. */
    labelId: string;
    options: Option[];
    value: string | undefined;
    onpick: (id: string) => void;
    /** What each option would cost, by option id: "about 25 min". */
    hints?: Record<string, string>;
  }
  let { labelId, options, value, onpick, hints = {} }: Props = $props();

  // A radio group is one stop on Tab; the arrows move inside it and choose as they go.
  function onkeydown(event: KeyboardEvent) {
    const delta = event.key === "ArrowRight" || event.key === "ArrowDown" ? 1 : event.key === "ArrowLeft" || event.key === "ArrowUp" ? -1 : 0;
    if (delta === 0) return;
    event.preventDefault();
    const at = Math.max(0, options.findIndex((o) => o.id === value));
    const next = options[(at + delta + options.length) % options.length];
    onpick(next.id);
    (event.currentTarget as HTMLElement).parentElement?.querySelectorAll<HTMLElement>('[role="radio"]')[options.indexOf(next)]?.focus();
  }
</script>

<div class="choices" style:--count={options.length} role="radiogroup" aria-labelledby={labelId}>
  {#each options as option (option.id)}
    <button type="button" role="radio" aria-checked={value === option.id} tabindex={value === option.id || (value === undefined && option === options[0]) ? 0 : -1} class="choice" {onkeydown} class:on={value === option.id} onclick={() => onpick(option.id)}>
      <span class="name">{option.name}</span>
      <span class="how">{option.how}</span>
      {#if hints[option.id]}<span class="hint">{hints[option.id]} for your folders</span>{/if}
    </button>
  {/each}
</div>

<style>
  .choices {
    display: grid;
    grid-template-columns: repeat(var(--count, 3), minmax(0, 1fr));
    gap: 8px;
  }
  .choice {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 12px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: transparent;
    color: var(--text-2);
    text-align: left;
    cursor: pointer;
    transition: background-color 0.12s, border-color 0.12s, color 0.12s;
  }
  .choice:hover {
    background: var(--tile);
  }
  .choice:active {
    background: var(--lift);
  }
  .choice.on {
    border-color: rgba(230, 232, 236, 0.45);
    background: var(--lift);
    color: var(--text);
  }
  .choice:focus-visible {
    outline: 1px solid var(--chrome);
    outline-offset: 2px;
  }
  .name {
    font: 620 13px/1.2 var(--sans);
  }
  .how {
    font-size: 11.5px;
    line-height: 1.45;
    color: var(--text-3);
  }
  .hint {
    margin-top: auto;
    padding-top: 4px;
    font: 560 11.5px/1.3 var(--sans);
    color: var(--text-2);
  }
</style>
