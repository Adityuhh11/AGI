<script lang="ts">
  import { onMount } from "svelte";
  import { Service as Settings } from "../../../bindings/omni/internal/settingsui";
  import type { FolderChoice, SearchView } from "../../../bindings/omni/internal/settingsui";
  import { isInside } from "../path";
  import { surveyLine } from "../settings/hints";

  interface Props {
    search: SearchView;
    /** Asks the guide to read the settings again after a change. */
    refresh: () => Promise<void>;
  }
  let { search, refresh }: Props = $props();

  let rows = $state<FolderChoice[]>([]);
  let error = $state("");

  // Ticking is saving, as everywhere in Settings. Nothing is read until indexing is started.
  async function apply() {
    error = await Settings.SetRoots(rows.filter((r) => r.Checked).map((r) => r.Path));
    await refresh();
  }

  async function load() {
    rows = (await Settings.SuggestedFolders()) ?? [];
  }

  // The usual folders start ticked, so opening this screen already saves them: leaving it unticked would be a choice too.
  onMount(async () => {
    await load();
    await apply();
  });

  async function add() {
    error = await Settings.AddFolder();
    await load();
    await refresh();
  }

  async function leaveOut(row: FolderChoice) {
    error = await Settings.LeaveOut(row.Path);
    await refresh();
  }

  async function putBack(folder: string) {
    error = await Settings.StopLeavingOut(folder);
    await refresh();
  }

  const leftOut = (row: FolderChoice) => (search.Exclude ?? []).filter((f) => isInside(row.Path, f));
  const holds = (row: FolderChoice) => surveyLine((search.Surveys ?? []).find((s) => isInside(row.Path, s.Root) && isInside(s.Root, row.Path)));
</script>

<h2>What should Omni search?</h2>
<p class="lede">
  Pick folders, and leave out the subfolders you do not need. The first pass reads everything once; after that Omni keeps up with changes as they
  happen.
</p>

<ul class="rows">
  {#each rows as row (row.Path)}
    <li>
      <div class="main">
        <label class="check">
          <input type="checkbox" bind:checked={row.Checked} onchange={apply} />
          <span class="name">{row.Name}</span>
        </label>
        <span class="path mono" title={row.Path}>{row.Path}</span>
        {#if row.Checked && row.Blocked}
          <span class="holds">
            macOS has not let Omni read this folder yet.
            <button type="button" class="btn quiet" onclick={() => Settings.OpenFileAccessSettings()}>Open System Settings</button>
          </span>
        {:else if row.Checked && holds(row)}<span class="holds">{holds(row)}</span>{/if}
        {#each leftOut(row) as folder (folder)}
          <span class="out">
            Left out: <span class="mono">{folder}</span>
            <button type="button" class="btn quiet" onclick={() => putBack(folder)}>Put back</button>
          </span>
        {/each}
      </div>
      <button type="button" class="btn quiet" disabled={!row.Checked} onclick={() => leaveOut(row)}>Leave out...</button>
    </li>
  {/each}
</ul>

<div><button type="button" class="btn" onclick={add}>Add folder...</button></div>
{#if error}<p class="error">{error}</p>{/if}
<p class="help note">Omni only reads files: it never changes, moves or uploads them. Secrets such as <span class="mono">.env</span> files and keys are never read.</p>

<style>
  .rows {
    display: flex;
    flex-direction: column;
    margin: 0 0 14px;
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
    padding: 8px 8px 8px 14px;
  }
  li + li {
    border-top: 1px solid var(--line);
  }
  .main {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
  }
  .name {
    font: 600 13.5px/1.2 var(--sans);
  }
  .path {
    padding-left: 26px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-3);
    font-size: 11.5px;
  }
  .holds {
    padding-left: 26px;
    color: var(--text-2);
    font-size: 12px;
  }
  .out {
    padding-left: 26px;
    color: var(--text-2);
    font-size: 11.5px;
  }
  .note {
    margin-top: 16px;
  }
</style>
