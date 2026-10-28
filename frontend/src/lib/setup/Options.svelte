<script lang="ts">
  import { Service as Settings } from "../../../bindings/omni/internal/settingsui";
  import type { SearchView } from "../../../bindings/omni/internal/settingsui";
  import ChoiceGroup from "../settings/ChoiceGroup.svelte";
  import { CHOICES, SPEEDS } from "../settings/choices";
  import { choiceHints, paceHints } from "../settings/hints";
  import type { Kind } from "../indexingPlan";

  interface Props {
    search: SearchView;
    refresh: () => Promise<void>;
  }
  let { search, refresh }: Props = $props();

  let error = $state("");

  // An index that exists is changed on the File search page, where a change is saved before it re-indexes.
  const existing = $derived(search.Indexed > 0 && !search.HoldIndexing);

  async function choose(kind: Kind, id: string) {
    const next = { PDFPages: search.PDFPages, Photos: search.Photos, Videos: search.Videos, Audio: search.Audio, [kind]: id };
    error = await Settings.SetIndexing(next.PDFPages, next.Photos, next.Videos, next.Audio);
    await refresh();
  }
</script>

<h2>How thorough?</h2>
<p class="lede">
  Pictures are what indexing time goes on: about 0.16 seconds each on a graphics card, about 5 seconds on a processor alone. Sound is
  cheaper: about a minute per hour of audio on a graphics card, 15 minutes on a processor.
</p>

{#if existing}
  <p class="help">
    You already have an index. Change these in Settings > File search, where a change is saved before it re-indexes.
  </p>
{:else}
  {#if search.Device === "cpu"}
    <p class="help intro warn">
      This computer has no graphics card file search can use, so it runs on the processor: each picture takes about 5 seconds. Words only and names
      are chosen for you; the dearer choices are there if you want them.
    </p>
  {:else if search.Device}
    <p class="help intro">It runs on your graphics card.</p>
  {:else}
    <p class="help intro">The search engine is still downloading; it will say here whether it found a graphics card.</p>
  {/if}

  {#each CHOICES as choice (choice.kind)}
    <div class="field">
      <span class="label" id="setup-{choice.kind}">{choice.label}</span>
      <ChoiceGroup labelId="setup-{choice.kind}" options={choice.options} value={search[choice.kind]} onpick={(id) => choose(choice.kind, id)} hints={choiceHints(choice.kind, choice.options.map((o) => o.id), search.Estimate)} />
    </div>
  {/each}

  <div class="field">
    <span class="label" id="setup-pace">How hard it works</span>
    <ChoiceGroup
      labelId="setup-pace"
      options={SPEEDS}
      value={search.Pace}
      hints={paceHints(search.Estimate)}
      onpick={async (id) => {
        error = await Settings.SetPace(id);
        await refresh();
      }}
    />
  </div>
  {#if error}<p class="error">{error}</p>{/if}
{/if}

<style>
  .intro {
    margin-bottom: 18px;
  }
  .warn {
    color: var(--danger);
  }
</style>
