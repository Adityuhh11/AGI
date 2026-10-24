<script lang="ts">
  import type { Option } from "./choices";

  interface Props {
    /** The id of the element that names the group. */
    labelId: string;
    options: Option[];
    value: string | undefined;
    onpick: (id: string) => void;
  }
  let { labelId, options, value, onpick }: Props = $props();
</script>

<div class="choices" style:--count={options.length} role="radiogroup" aria-labelledby={labelId}>
  {#each options as option (option.id)}
    <button type="button" role="radio" aria-checked={value === option.id} class="choice" class:on={value === option.id} onclick={() => onpick(option.id)}>
      <span class="name">{option.name}</span>
      <span class="how">{option.how}</span>
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
  }
  .choice:hover {
    background: var(--tile);
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
</style>
