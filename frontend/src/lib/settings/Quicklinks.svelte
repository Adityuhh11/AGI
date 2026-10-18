<script lang="ts">
  import { Service as Settings } from "../../../bindings/agi/internal/settingsui";
  import type { View } from "../../../bindings/agi/internal/settingsui";

  let { view }: { view: View } = $props();

  type Row = { trigger: string; title: string; url: string };
  const complete = (r: Row) => r.trigger.trim() !== "" && r.title.trim() !== "" && r.url.trim() !== "";
  const body = (list: Row[]) =>
    list.filter(complete).map((r) => ({ Trigger: r.trigger.trim(), Title: r.title.trim(), URL: r.url.trim() }));

  // The table starts from the quicklinks as they were when the page opened and then owns its rows.
  // svelte-ignore state_referenced_locally
  let rows = $state<Row[]>((view.Quicklinks ?? []).map((l) => ({ trigger: l.Trigger, title: l.Title, url: l.URL })));
  let lastSaved = JSON.stringify(body(rows));
  let error = $state("");
  let saved = $state(false);
  let savedTimer: ReturnType<typeof setTimeout> | undefined;
  const incomplete = $derived(rows.some((r) => !complete(r) && (r.trigger || r.title || r.url)));
  const QUERY = "{q}";

  // A row is saved once all three of its fields are filled in; until then it stays on this page only.
  async function commit() {
    const links = body(rows);
    const text = JSON.stringify(links);
    if (text === lastSaved) return;
    error = await Settings.SaveQuicklinks(links);
    if (error !== "") return;
    lastSaved = text;
    saved = true;
    clearTimeout(savedTimer);
    savedTimer = setTimeout(() => (saved = false), 1800);
  }

  function remove(index: number) {
    rows.splice(index, 1);
    commit();
  }
</script>

<h2>Quicklinks</h2>
<p class="lede">
  Type a trigger, a space, and what to look for: <span class="mono">gh rust lifetimes</span> opens that search on GitHub. Put {QUERY} in the
  address where the words go.
</p>

{#if rows.length > 0}
  <div class="table" role="group" aria-label="Quicklinks">
    <div class="head"><span>Trigger</span><span>Name</span><span>Address</span><span></span></div>
    {#each rows as row, i (i)}
      <div class="row">
        <input type="text" class="mono" aria-label="Trigger" placeholder="gh" bind:value={row.trigger} onchange={commit} spellcheck="false" />
        <input type="text" aria-label="Name" placeholder="GitHub" bind:value={row.title} onchange={commit} />
        <input
          type="text"
          class="mono"
          aria-label="Address"
          placeholder={"https://github.com/search?q=" + QUERY}
          bind:value={row.url}
          onchange={commit}
          spellcheck="false"
        />
        <button type="button" class="btn quiet" aria-label="Remove this quicklink" onclick={() => remove(i)}>✕</button>
      </div>
    {/each}
  </div>
{:else}
  <p class="help">No quicklinks yet.</p>
{/if}

<div class="actions">
  <button type="button" class="btn" onclick={() => rows.push({ trigger: "", title: "", url: "" })}>Add quicklink</button>
</div>
{#if incomplete}<p class="help">A quicklink is saved once its trigger, name and address are all filled in.</p>{/if}
{#if error}<p class="error">{error}</p>{/if}
<p class="ok" aria-live="polite">{saved ? "Saved" : ""}</p>

<style>
  .table {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 14px;
  }
  .head,
  .row {
    display: grid;
    grid-template-columns: 84px 120px minmax(0, 1fr) 34px;
    gap: 8px;
    align-items: center;
  }
  .head {
    color: var(--text-3);
    font: 500 11.5px/1 var(--sans);
    padding: 0 2px;
  }
  .row :global(input) {
    min-width: 0;
  }
  .actions {
    margin-bottom: 10px;
  }
</style>
