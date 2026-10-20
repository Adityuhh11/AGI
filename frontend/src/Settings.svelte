<script lang="ts">
  import { onMount } from "svelte";
  import { Service as Settings } from "../bindings/agi/internal/settingsui";
  import type { View } from "../bindings/agi/internal/settingsui";
  import About from "./lib/settings/About.svelte";
  import Clipboard from "./lib/settings/Clipboard.svelte";
  import FileSearch from "./lib/settings/FileSearch.svelte";
  import General from "./lib/settings/General.svelte";
  import Quicklinks from "./lib/settings/Quicklinks.svelte";
  import Snippets from "./lib/settings/Snippets.svelte";

  const SECTIONS = [
    { id: "general", label: "General" },
    { id: "quicklinks", label: "Quicklinks" },
    { id: "snippets", label: "Snippets" },
    { id: "search", label: "File search" },
    { id: "clipboard", label: "Clipboard" },
    { id: "about", label: "About" },
  ] as const;

  // A window opened on a section ("#settings/about", after an update) starts there.
  const wanted = location.hash.split("/")[1];
  let section = $state<(typeof SECTIONS)[number]["id"]>(SECTIONS.find((s) => s.id === wanted)?.id ?? "general");
  let view = $state<View | null>(null);
  const reload = async () => {
    view = await Settings.View();
  };

  onMount(() => {
    document.documentElement.dataset.surface = "settings";
    reload();
  });
</script>

<div class="settings">
  <nav aria-label="Settings sections">
    <h1>Settings</h1>
    {#each SECTIONS as s (s.id)}
      <button type="button" class:on={section === s.id} aria-current={section === s.id ? "page" : undefined} onclick={() => (section = s.id)}>
        {s.label}
      </button>
    {/each}
  </nav>
  <main>
    {#if view}
      {#if section === "general"}
        <General {view} />
      {:else if section === "quicklinks"}
        <Quicklinks {view} />
      {:else if section === "snippets"}
        <Snippets {view} />
      {:else if section === "search"}
        <FileSearch {view} {reload} />
      {:else if section === "clipboard"}
        <Clipboard />
      {:else}
        <About {view} />
      {/if}
    {/if}
  </main>
</div>

<style>
  .settings {
    display: grid;
    grid-template-columns: 196px minmax(0, 1fr);
    min-height: 100vh;
    background: var(--plate);
    color: var(--text);
  }
  nav {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 22px 12px;
    border-right: 1px solid var(--line);
  }
  nav h1 {
    margin: 0 0 14px;
    padding: 0 12px;
    font: 620 15px/1.2 var(--sans);
    letter-spacing: -0.01em;
  }
  nav button {
    height: 36px;
    padding: 0 12px;
    border: 0;
    border-radius: 10px;
    background: transparent;
    color: var(--text-2);
    font: 500 13.5px/1 var(--sans);
    text-align: left;
    cursor: pointer;
  }
  nav button:hover {
    color: var(--text);
  }
  nav button.on {
    background: var(--lift);
    color: var(--text);
    font-weight: 600;
  }
  nav button:focus-visible {
    outline: 1px solid var(--chrome);
    outline-offset: -1px;
  }
  main {
    padding: 28px 40px 48px;
    max-width: 640px;
  }

  /* Shared by every section. */
  main :global(h2) {
    margin: 0 0 4px;
    font: 620 20px/1.25 var(--sans);
    letter-spacing: -0.015em;
  }
  main :global(.lede) {
    margin: 0 0 24px;
    color: var(--text-2);
    font-size: 13px;
    line-height: 1.5;
  }
  main :global(.field) {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 22px;
  }
  main :global(.field > .label) {
    color: var(--text);
    font: 560 13px/1.2 var(--sans);
  }
  main :global(.check) {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--text);
    font: 560 13px/1.2 var(--sans);
    cursor: pointer;
  }
  main :global(.check input) {
    accent-color: var(--text-2);
  }
  main :global(.help) {
    margin: 0;
    color: var(--text-3);
    font-size: 12px;
    line-height: 1.45;
  }
  main :global(.error) {
    margin: 0;
    color: var(--danger);
    font-size: 12.5px;
    line-height: 1.45;
  }
  main :global(.ok) {
    margin: 0;
    color: var(--text-3);
    font-size: 12px;
  }
  main :global(input[type="text"]),
  main :global(select) {
    height: 36px;
    padding: 0 12px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--tile);
    color: var(--text);
    font: 450 13.5px/1 var(--sans);
  }
  main :global(select) {
    appearance: none;
    background-image: linear-gradient(45deg, transparent 50%, var(--text-3) 50%), linear-gradient(135deg, var(--text-3) 50%, transparent 50%);
    background-position: calc(100% - 17px) 15px, calc(100% - 12px) 15px;
    background-size: 5px 5px;
    background-repeat: no-repeat;
    padding-right: 32px;
  }
  main :global(select option) {
    background: #111;
    color: var(--text);
  }
  main :global(input[type="text"]:focus),
  main :global(select:focus) {
    outline: none;
    border-color: rgba(230, 232, 236, 0.45);
  }
  main :global(input::placeholder) {
    color: var(--text-3);
  }
  main :global(.mono) {
    font-family: var(--mono);
    font-size: 12.5px;
  }
  main :global(.btn) {
    height: 34px;
    padding: 0 14px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--tile);
    color: var(--text);
    font: 560 13px/1 var(--sans);
    cursor: pointer;
  }
  main :global(.btn:hover:not(:disabled)) {
    background: var(--lift);
  }
  main :global(.btn:disabled) {
    color: var(--text-3);
    cursor: default;
  }
  main :global(.btn.primary) {
    border-color: transparent;
    background: var(--chrome);
    color: #000;
    font-weight: 620;
  }
  main :global(.btn.primary:disabled) {
    background: var(--tile);
    color: var(--text-3);
  }
  main :global(.btn.quiet) {
    border-color: transparent;
    background: transparent;
    color: var(--text-2);
  }
  main :global(.btn.danger) {
    color: var(--danger);
  }
  main :global(.btn:focus-visible),
  main :global(.keys:focus-visible) {
    outline: 1px solid var(--chrome);
    outline-offset: 2px;
  }
  main :global(kbd) {
    display: inline-block;
    min-width: 22px;
    padding: 3px 8px;
    border-radius: 7px;
    background: var(--tile);
    color: var(--text);
    font: 500 12px/1.2 var(--mono);
    text-align: center;
  }
</style>
