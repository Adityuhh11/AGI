<script lang="ts">
  interface Props {
    /** The hours as saved ("22:00-07:00"), or "" when indexing may work at any hour. */
    value: string;
    /** Saves the hours ("" turns the limit off). */
    onsave: (between: string) => Promise<void>;
  }
  let { value, onsave }: Props = $props();

  // The fields start from what was saved when the page opened and then own what they show; the page refreshes every second.
  // svelte-ignore state_referenced_locally
  let [from, to] = $state(value ? value.split("-") : ["22:00", "07:00"]);
  // svelte-ignore state_referenced_locally
  let on = $state(value !== "");

  const save = () => (on && (!from || !to || from === to) ? Promise.resolve() : onsave(on ? `${from}-${to}` : ""));
</script>

<label class="check">
  <input type="checkbox" bind:checked={on} onchange={save} />
  Only index between
  <input type="time" class="time" bind:value={from} disabled={!on} onchange={save} aria-label="From" />
  and
  <input type="time" class="time" bind:value={to} disabled={!on} onchange={save} aria-label="To" />
</label>
<p class="help">
  Outside these hours indexing waits and the tray says when it goes on; searching is not affected. Past midnight is fine (22:00 to 07:00).
  Files saved meanwhile are picked up when the hours begin.
</p>

<style>
  .time {
    width: auto;
    margin: 0 4px;
    padding: 4px 8px;
  }
</style>
