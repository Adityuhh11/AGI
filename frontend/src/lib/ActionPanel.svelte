<script lang="ts">
  import { combo } from "./platform";
  import { onMount } from "svelte";
  import { filterActions } from "./actions";
  import { step } from "./flatten";

  /** One line of the panel: an action of a row, or a tag to put on it. */
  type Line = { id: string; title: string; keys?: string };

  interface Props {
    actions: readonly Line[];
    onrun: (id: string) => void;
    onclose: () => void;
    placeholder?: string;
    /** A line offered for the text typed when nothing in the list is it (a new tag), or null. */
    create?: (text: string) => Line | null;
  }
  let { actions, onrun, onclose, placeholder = "Filter actions", create }: Props = $props();

  let text = $state("");
  let selected = $state(0);
  let field: HTMLInputElement;

  const shown = $derived.by(() => {
    const made = create?.(text.trim());
    return [...filterActions(actions, text), ...(made ? [made] : [])];
  });
  // Typing can shorten the list under the selection.
  $effect(() => {
    if (selected >= shown.length) selected = Math.max(0, shown.length - 1);
  });

  onMount(() => field.focus());

  // The panel owns the keyboard while it is open: nothing here may reach the bar's own key handling,
  // where Enter would open the file and Escape would close the whole bar.
  function onkeydown(event: KeyboardEvent) {
    const close = event.key === "Escape" || (event.ctrlKey && event.key.toLowerCase() === "k");
    if (close) onclose();
    else if (event.key === "ArrowDown") selected = step(selected, 1, shown.length);
    else if (event.key === "ArrowUp") selected = step(selected, -1, shown.length);
    else if (event.key === "Enter") {
      if (shown[selected]) onrun(shown[selected].id);
    } else return; // typing goes to the filter
    event.preventDefault();
    event.stopPropagation();
  }
</script>

<div class="panel" role="dialog" aria-label="Actions">
  <input
    bind:this={field}
    bind:value={text}
    {onkeydown}
    type="text"
    {placeholder}
    spellcheck="false"
    autocomplete="off"
    aria-label={placeholder}
    role="combobox"
    aria-expanded="true"
    aria-controls="action-list"
    aria-activedescendant={shown.length > 0 ? "action-" + selected : undefined}
  />
  <ul role="listbox" aria-label="Actions" id="action-list">
    {#each shown as action, index (action.id)}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <li
        id={"action-" + index}
        role="option"
        aria-selected={index === selected}
        class:selected={index === selected}
        onmousemove={() => (selected = index)}
        onclick={() => onrun(action.id)}
      >
        <span>{action.title}</span>
        {#if action.keys}<kbd>{combo(action.keys)}</kbd>{/if}
      </li>
    {:else}
      <li class="none">Nothing here</li>
    {/each}
  </ul>
</div>

<style>
  .panel {
    position: absolute;
    right: 12px;
    bottom: 50px;
    z-index: 5;
    width: 290px;
    padding: 6px;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 16px;
    background: var(--plate);
    box-shadow: 0 14px 30px rgba(0, 0, 0, 0.7);
  }
  input {
    box-sizing: border-box;
    width: 100%;
    height: 34px;
    margin-bottom: 4px;
    padding: 0 10px;
    border: 0;
    border-bottom: 1px solid var(--line);
    outline: 0;
    background: transparent;
    color: var(--text);
    font: 450 13px var(--sans);
  }
  input::placeholder {
    color: var(--text-3);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    height: 34px;
    padding: 0 10px;
    border-radius: 10px;
    font-size: 13px;
  }
  li.selected {
    background: var(--lift);
  }
  li:not(.none) {
    cursor: pointer;
  }
  li.none {
    color: var(--text-3);
  }
  kbd {
    color: var(--text-3);
    font: 500 10.5px var(--mono);
  }
</style>
