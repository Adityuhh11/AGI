<script lang="ts">
  import { Service as Settings } from "../../../bindings/omni/internal/settingsui";
  import type { View } from "../../../bindings/omni/internal/settingsui";

  let { view }: { view: View } = $props();

  type Row = { name: string; keyword: string; text: string };
  const complete = (r: Row) => r.name.trim() !== "" && r.text !== "";
  const body = (list: Row[]) => list.filter(complete).map((r) => ({ Name: r.name.trim(), Keyword: r.keyword.trim(), Text: r.text }));

  // The list starts from the snippets as they were when the page opened and then owns its rows.
  // svelte-ignore state_referenced_locally
  let rows = $state<Row[]>((view.Snippets ?? []).map((s) => ({ name: s.Name, keyword: s.Keyword, text: s.Text })));
  let lastSaved = JSON.stringify(body(rows));
  let error = $state("");
  let saved = $state(false);
  let savedTimer: ReturnType<typeof setTimeout> | undefined;
  const incomplete = $derived(rows.some((r) => !complete(r) && (r.name || r.keyword || r.text)));

  // A snippet is saved once it has a name and a text; until then it stays on this page only.
  async function commit() {
    const snippets = body(rows);
    const text = JSON.stringify(snippets);
    if (text === lastSaved) return;
    error = await Settings.SaveSnippets(snippets);
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

<h2>Snippets</h2>
<p class="lede">
  Text you paste often. Open the bar, type <span class="mono">snip:</span> and pick one, or type its keyword. Enter pastes it where you were.
  <span class="mono">{"{date}"}</span>, <span class="mono">{"{time}"}</span> and <span class="mono">{"{clipboard}"}</span> are filled in when it is
  pasted; <span class="mono">{"{{"}</span> writes a single brace.
</p>

{#each rows as row, i (i)}
  <div class="snippet">
    <div class="top">
      <input type="text" aria-label="Name" placeholder="Home address" bind:value={row.name} onchange={commit} />
      <input type="text" class="mono" aria-label="Keyword" placeholder="keyword (optional)" bind:value={row.keyword} onchange={commit} spellcheck="false" />
      <button type="button" class="btn quiet" aria-label="Remove this snippet" onclick={() => remove(i)}>✕</button>
    </div>
    <textarea aria-label="Text" rows="3" placeholder="The text to paste" bind:value={row.text} onchange={commit} spellcheck="false"></textarea>
  </div>
{:else}
  <p class="help">No snippets yet.</p>
{/each}

<div class="actions">
  <button type="button" class="btn" onclick={() => rows.push({ name: "", keyword: "", text: "" })}>Add snippet</button>
</div>
{#if incomplete}<p class="help">A snippet is saved once it has a name and a text.</p>{/if}
{#if error}<p class="error">{error}</p>{/if}
<p class="ok" aria-live="polite">{saved ? "Saved" : ""}</p>

<style>
  .snippet {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 14px;
    padding: 12px;
    border: 1px solid var(--line);
    border-radius: 12px;
  }
  .top {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 180px 34px;
    gap: 8px;
    align-items: center;
  }
  .top :global(input) {
    min-width: 0;
  }
  textarea {
    box-sizing: border-box;
    width: 100%;
    padding: 9px 12px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--tile);
    color: var(--text);
    font: 450 13px/1.5 var(--mono);
    resize: vertical;
  }
  textarea:focus {
    outline: none;
    border-color: color-mix(in srgb, var(--chrome) 45%, transparent);
  }
  textarea::placeholder {
    color: var(--text-3);
  }
  .actions {
    margin-bottom: 10px;
  }
</style>
