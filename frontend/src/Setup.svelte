<script lang="ts">
  import { onMount } from "svelte";
  import { Window } from "@wailsio/runtime";
  import { Service as Settings } from "../bindings/omni/internal/settingsui";
  import type { SearchView, View } from "../bindings/omni/internal/settingsui";
  import "./lib/settings/forms.css";
  import { after, before, type Step } from "./lib/setup/flow";
  import Done from "./lib/setup/Done.svelte";
  import Files from "./lib/setup/Files.svelte";
  import Folders from "./lib/setup/Folders.svelte";
  import Keys from "./lib/setup/Keys.svelte";
  import Options from "./lib/setup/Options.svelte";
  import Welcome from "./lib/setup/Welcome.svelte";

  let step = $state<Step>("welcome");
  let view = $state<View | null>(null);
  let search = $state<SearchView | null>(null);
  // Whether the person is setting up file search: they pressed Download or Turn on, or it was already running.
  let files = $state(false);
  let enableError = $state("");

  const refreshSearch = async () => {
    search = await Settings.Search();
  };
  const refresh = async () => {
    [view, search] = await Promise.all([Settings.View(), Settings.Search()]);
  };

  // While a download runs the guide asks every second how it is going; the footer shows it on every screen.
  onMount(() => {
    document.documentElement.dataset.surface = "settings";
    refresh().then(() => {
      if (search && (search.Installing || (search.Installed && search.Enabled))) files = true;
    });
    const timer = setInterval(refreshSearch, 1000);
    return () => clearInterval(timer);
  });

  // A computer with no graphics card gets the cheap choices, once, as long as the person has not changed any: pictures
  // take seconds each there, and the dear choices are one click away.
  let cheapened = false;
  $effect(() => {
    if (!search || cheapened || search.Device !== "cpu" || !search.HoldIndexing) return;
    if (search.PDFPages !== "auto" || search.Photos !== "content" || search.Videos !== "name") return;
    cheapened = true;
    Settings.SetIndexing("text", "name", "name").then(refreshSearch);
  });

  async function enable() {
    enableError = await Settings.TurnOn();
    if (enableError !== "") return;
    files = true;
    await refreshSearch();
    step = after("files", true);
  }

  // Choosing where the download goes can be refused (a folder that cannot be written); the reason shows where the button is.
  async function chooseFolder(change: () => Promise<string>) {
    enableError = await change();
    await refreshSearch();
  }

  function skip() {
    files = false;
    step = "done";
  }

  const mb = (bytes: number) => (bytes / (1024 * 1024)).toFixed(0);
  const footer = $derived.by((): { text: string; problem: boolean } => {
    if (!search || !files) return { text: "", problem: false };
    if (search.Installing) {
      const part = search.Total > 0 ? ` · ${mb(search.Done)} of ${mb(search.Total)} MB` : "";
      return { text: `Downloading file search · step ${search.Step} of ${search.Steps}${part}`, problem: false };
    }
    if (search.Error) return { text: "The download failed: " + search.Error, problem: true };
    if (search.Installed && search.Enabled) {
      return { text: search.EngineState === "ready" ? "File search is ready" : "Starting the search engine...", problem: false };
    }
    return { text: "", problem: false };
  });

  // The files screen has its own buttons (Download, Not now), so Continue shows there only once a download has started.
  const canContinue = $derived(step !== "files" || !!search?.Installing || (!!search?.Installed && !!search?.Enabled));
</script>

<main class="setup">
  <div class="content">
    {#if view && search}
      {#if step === "welcome"}
        <Welcome />
      {:else if step === "keys"}
        <Keys {view} />
      {:else if step === "files"}
        <Files {search} error={enableError} onenable={enable} onskip={skip} onchoose={() => chooseFolder(Settings.ChooseDataFolder)} onreset={() => chooseFolder(Settings.UseDefaultDataFolder)} />
      {:else if step === "folders"}
        <Folders {search} {refresh} />
      {:else if step === "options"}
        <Options {search} refresh={refreshSearch} />
      {:else}
        <Done {view} {search} {files} {refresh} onfolders={() => (step = "folders")} />
      {/if}
    {/if}
  </div>

  <footer>
    <span class="status" class:problem={footer.problem} aria-live="polite">{footer.text}</span>
    <div class="nav">
      {#if step !== "welcome"}
        <button type="button" class="btn" onclick={() => (step = before(step, files))}>Back</button>
      {/if}
      {#if step === "done"}
        <button type="button" class="btn primary" onclick={() => Window.Close()}>Done</button>
      {:else if canContinue}
        <button type="button" class="btn primary" onclick={() => (step = after(step, files))}>Continue</button>
      {/if}
    </div>
  </footer>
</main>

<style>
  .setup {
    display: flex;
    flex-direction: column;
    height: 100vh;
    background: var(--plate);
    color: var(--text);
  }
  .content {
    flex: 1;
    min-height: 0;
    padding: 36px 48px 24px;
    overflow-y: auto;
  }
  footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    min-height: 64px;
    padding: 0 24px 0 48px;
    border-top: 1px solid var(--line);
  }
  .status {
    color: var(--text-3);
    font-size: 12.5px;
  }
  .status.problem {
    color: var(--danger);
  }
  .nav {
    display: flex;
    gap: 8px;
  }
</style>
