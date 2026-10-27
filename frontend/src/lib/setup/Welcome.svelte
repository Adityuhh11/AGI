<script lang="ts">
  import Icon from "../Icon.svelte";
</script>

<h2>Welcome to Omni</h2>
<p class="lede">Press a hotkey, type, and open anything. It takes a minute to set up, and you can skip every part of it.</p>

<ul class="rows">
  <li>
    <span class="tile"><Icon name="app" /></span>
    <span>
      <strong>Apps and the web, at once</strong>
      <span class="eg">“vsc”, “chr”, “gh rust lifetimes”</span>
    </span>
  </li>
  <li>
    <span class="tile"><Icon name="file" /></span>
    <span>
      <strong>Your files, by what they say <em>(optional)</em></strong>
      <span class="eg">“the diagram with three boxes”, “t distribution table”, “dog on a beach”</span>
    </span>
  </li>
  <li>
    <span class="tile"><Icon name="clip" /></span>
    <span>
      <strong>Clipboard history, snippets and pins</strong>
      <span class="eg">“clip: invoice”, “snip: address”</span>
    </span>
  </li>
</ul>

<p class="help">
  Everything runs on this computer: no account and no telemetry. Omni uses the network only when you ask it to: the optional
  file-search download (about 1 GB, once), a web search in your browser, and an update check if you switch it on.
</p>

<style>
  .rows {
    display: flex;
    flex-direction: column;
    gap: 18px;
    margin: 8px 0 28px;
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    gap: 14px;
  }
  .tile {
    display: grid;
    place-items: center;
    flex: none;
    width: 40px;
    height: 40px;
    border-radius: 10px;
    background: var(--tile);
    color: var(--text-2);
  }
  strong {
    display: block;
    font: 600 14px/1.3 var(--sans);
  }
  em {
    color: var(--text-3);
    font-style: normal;
    font-weight: 450;
  }
  .eg {
    color: var(--text-3);
    font-size: 12.5px;
  }
</style>
