<script lang="ts">
  import { onMount } from "svelte";
  import { Service as Settings } from "../bindings/omni/internal/settingsui";
  import type { View } from "../bindings/omni/internal/settingsui";
  import "./lib/settings/forms.css";
  import { Window } from "@wailsio/runtime";
  import Card from "./lib/Card.svelte";
  import RoundButton from "./lib/RoundButton.svelte";
  import About from "./lib/settings/About.svelte";
  import Clipboard from "./lib/settings/Clipboard.svelte";
  import FileSearch from "./lib/settings/FileSearch.svelte";
  import General from "./lib/settings/General.svelte";
  import OmniAI from "./lib/settings/OmniAI.svelte";
  import Quicklinks from "./lib/settings/Quicklinks.svelte";
  import Snippets from "./lib/settings/Snippets.svelte";
  import TimerSettings from "./lib/settings/TimerSettings.svelte";

  const SECTIONS = [
    { id: "general", label: "General" },
    { id: "quicklinks", label: "Quicklinks" },
    { id: "snippets", label: "Snippets" },
    { id: "search", label: "File search" },
    { id: "clipboard", label: "Clipboard" },
    { id: "omni-ai", label: "Omni AI" },
    { id: "timer", label: "Timer" },
    { id: "about", label: "About" },
  ] as const;

  // A window opened on a section ("#settings/about", after an update) starts there.
  const wanted = location.hash.split("/")[1];
  let section = $state<(typeof SECTIONS)[number]["id"]>(SECTIONS.find((s) => s.id === wanted)?.id ?? "general");
  let view = $state<View | null>(null);
  const reload = async () => {
    view = await Settings.View();
  };

  // The bar can open this window on a page while it is already open: the address changes and the page follows.
  const follow = () => {
    const next = SECTIONS.find((s) => s.id === location.hash.split("/")[1]);
    if (next) section = next.id;
  };

  onMount(() => {
    document.documentElement.dataset.surface = "card";
    reload();
    window.addEventListener("hashchange", follow);
    return () => window.removeEventListener("hashchange", follow);
  });
</script>

<Card>
<div class="settings">
  <div class="drag" aria-hidden="true"></div>
  <div class="close"><RoundButton icon="close" label="Close" onclick={() => Window.Close()} /></div>
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
      {:else if section === "omni-ai"}
        <OmniAI />
      {:else if section === "timer"}
        <TimerSettings {view} />
      {:else}
        <About {view} />
      {/if}
    {/if}
  </main>
</div>
</Card>

<style>
  .settings {
    position: relative;
    display: grid;
    flex: 1;
    grid-template-columns: 196px minmax(0, 1fr);
    min-height: 0;
    color: var(--text);
  }
  /* The top of the window drags it; the close button sits above that strip. */
  .drag {
    position: absolute;
    top: 0;
    right: 0;
    left: 0;
    z-index: 1;
    height: 46px;
    --wails-draggable: drag;
  }
  .close {
    position: absolute;
    top: 10px;
    right: 14px;
    z-index: 2;
  }
  nav {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 24px 12px;
    border-right: 1px solid var(--line);
    overflow-y: auto;
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
    transition: background-color 0.12s, color 0.12s;
  }
  nav button:hover {
    background: var(--tile);
    color: var(--text);
  }
  nav button:active {
    background: var(--lift);
  }
  nav button.on,
  nav button.on:hover {
    background: var(--lift);
    color: var(--text);
    font-weight: 600;
  }
  nav button:focus-visible {
    outline: 1px solid var(--chrome);
    outline-offset: -1px;
  }
  main {
    min-height: 0;
    padding: 36px 40px 48px;
    overflow-y: auto;
  }
  main > :global(*) {
    max-width: 640px;
  }
</style>
