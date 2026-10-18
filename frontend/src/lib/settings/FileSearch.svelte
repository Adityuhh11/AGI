<script lang="ts">
  import { Service as Settings } from "../../../bindings/agi/internal/settingsui";
  import type { View } from "../../../bindings/agi/internal/settingsui";

  let { view, reload }: { view: View; reload: () => Promise<void> } = $props();

  let error = $state("");
  let confirming = $state("");
  const roots = $derived(view.Roots ?? []);

  async function add() {
    error = await Settings.AddFolder();
    await reload();
  }

  async function remove(folder: string) {
    confirming = "";
    error = await Settings.RemoveFolder(folder);
    await reload();
  }
</script>

<h2>File search</h2>
<p class="lede">
  Find files by what they contain: code, PDF pages, notes, and pictures by what they show. It runs on this computer and nothing leaves it.
</p>

<div class="field">
  <span class="label">Status</span>
  <p class="help">File search is not part of this build yet. The folders you choose below are kept for when it is.</p>
  <div><button type="button" class="btn primary" disabled>Turn on</button></div>
</div>

<div class="field">
  <span class="label">Folders to search</span>
  {#if roots.length > 0}
    <ul class="folders">
      {#each roots as root (root)}
        <li>
          <span class="path mono" title={root}>{root}</span>
          {#if confirming === root}
            <span class="confirm">
              <button type="button" class="btn danger" onclick={() => remove(root)}>Remove</button>
              <button type="button" class="btn quiet" onclick={() => (confirming = "")}>Keep</button>
            </span>
          {:else}
            <button type="button" class="btn quiet" onclick={() => (confirming = root)}>Remove</button>
          {/if}
        </li>
      {/each}
    </ul>
  {:else}
    <p class="help">No folders yet. Nothing is searched until you add one.</p>
  {/if}
  <div><button type="button" class="btn" onclick={add}>Add folder…</button></div>
  {#if error}<p class="error">{error}</p>{/if}
</div>

<style>
  .folders {
    display: flex;
    flex-direction: column;
    margin: 0;
    padding: 0;
    list-style: none;
    border: 1px solid var(--line);
    border-radius: 10px;
  }
  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 6px 6px 12px;
  }
  li + li {
    border-top: 1px solid var(--line);
  }
  .path {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    direction: rtl; /* a long path keeps its end, where the folder's own name is */
    text-align: left;
    color: var(--text-2);
  }
  .confirm {
    display: flex;
    gap: 4px;
  }
</style>
