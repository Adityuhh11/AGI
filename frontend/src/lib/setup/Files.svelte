<script lang="ts">
  import type { SearchView } from "../../../bindings/agi/internal/settingsui";

  interface Props {
    search: SearchView;
    /** Why the last attempt to start the download was refused, or "". */
    error: string;
    onenable: () => void;
    onskip: () => void;
  }
  let { search, error, onenable, onskip }: Props = $props();

  const gb = $derived((search.DownloadMB / 1024).toFixed(1));
  // Installing, or installed and on. Installed and off still needs a button: turning it on starts the engine.
  const started = $derived(search.Installing || (search.Installed && search.Enabled));
</script>

<h2>Search your files too?</h2>
<p class="lede">
  File search finds a file by what is in it, not by its name: a page of a PDF, a function, a photo. The launcher works without it, and
  you can turn it on later in Settings.
</p>

<ul class="eg">
  <li>“the clause about pets in the lease” opens the PDF page</li>
  <li>“retry a failed call with backoff” finds the function; <kbd>code:</kbd> limits a search to code</li>
  <li>“red ball floating in a river” finds the video, and “dog on a beach” the photo</li>
</ul>

<div class="field">
  <span class="label">What it costs</span>
  <ul class="costs">
    <li><strong>A download of about {gb} GB, once</strong>: a search engine and its model. After that it works offline and sends nothing anywhere.</li>
    <li>
      <strong>About 1.1 GB of disk</strong>, saved in <span class="mono where">{search.EngineDir}</span>. The index it builds is kept next to it, in
      <span class="mono">%LOCALAPPDATA%\AGI\memory</span>.
    </li>
    <li>It uses your graphics card when it can. On the processor a picture takes about 5 seconds instead of a fraction of one; the guide says which you have, and nothing is indexed until you start it.</li>
  </ul>
  <p class="help mono">From {(search.Hosts ?? []).join(", ")}</p>
</div>

{#if search.Error}<p class="error">The last attempt failed: {search.Error}</p>{/if}
{#if error}<p class="error">{error}</p>{/if}

<div class="actions">
  {#if started}
    <p class="help">{search.Installing ? "The download is running; you can carry on." : "File search is installed."}</p>
  {:else}
    <button type="button" class="btn primary" onclick={onenable}>{search.Installed ? "Turn on and continue" : "Download and continue"}</button>
    <button type="button" class="btn quiet" onclick={onskip}>Not now</button>
  {/if}
</div>

<style>
  .eg {
    margin: 0 0 22px;
    padding-left: 18px;
    color: var(--text-2);
    font-size: 13px;
    line-height: 1.7;
  }
  .costs {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin: 0;
    padding-left: 18px;
    color: var(--text-2);
    font-size: 13px;
    line-height: 1.5;
  }
  .costs strong {
    color: var(--text);
    font-weight: 600;
  }
  .where {
    overflow-wrap: anywhere;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }
</style>
