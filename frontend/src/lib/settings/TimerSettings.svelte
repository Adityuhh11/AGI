<script lang="ts">
  import { Service as Settings } from "../../../bindings/omni/internal/settingsui";
  import type { View } from "../../../bindings/omni/internal/settingsui";

  let { view }: { view: View } = $props();

  // The form starts from the settings as they were when the page opened and then owns its own values.
  // svelte-ignore state_referenced_locally
  let focus = $state(view.Timer.FocusMinutes);
  // svelte-ignore state_referenced_locally
  let short = $state(view.Timer.ShortBreakMinutes);
  // svelte-ignore state_referenced_locally
  let long = $state(view.Timer.LongBreakMinutes);
  // svelte-ignore state_referenced_locally
  let rounds = $state(view.Timer.Rounds);
  // svelte-ignore state_referenced_locally
  let auto = $state(view.Timer.AutoNext);
  // svelte-ignore state_referenced_locally
  let presets = $state((view.Timer.PresetMinutes ?? []).join(", "));
  let error = $state("");
  let saved = $state(false);
  let savedTimer: ReturnType<typeof setTimeout> | undefined;

  async function save() {
    const parts = presets.split(/[,\s]+/).filter(Boolean);
    const minutes = parts.map(Number);
    if (minutes.some((m) => !Number.isInteger(m))) {
      error = `“${parts.find((p) => !Number.isInteger(Number(p)))}” is not a whole number of minutes.`;
      return;
    }
    error = await Settings.SaveTimer({
      FocusMinutes: focus,
      ShortBreakMinutes: short,
      LongBreakMinutes: long,
      Rounds: rounds,
      AutoNext: auto,
      PresetMinutes: minutes,
    });
    if (error === "") {
      saved = true;
      clearTimeout(savedTimer);
      savedTimer = setTimeout(() => (saved = false), 1800);
    }
  }
</script>

<h2>Timer</h2>
<p class="lede">
  Type <span class="mono">timer 25</span> in the launcher for a countdown, or <span class="mono">pomodoro</span> for focus periods with breaks. The timer sits at the
  edge of the screen and keeps going when you close it.
</p>

<div class="grid">
  <label class="label" for="focus">Focus</label>
  <span class="unit"><input id="focus" type="number" min="1" max="180" bind:value={focus} onchange={save} /> minutes</span>
  <label class="label" for="short">Short break</label>
  <span class="unit"><input id="short" type="number" min="1" max="60" bind:value={short} onchange={save} /> minutes</span>
  <label class="label" for="long">Long break</label>
  <span class="unit"><input id="long" type="number" min="1" max="120" bind:value={long} onchange={save} /> minutes</span>
  <label class="label" for="rounds">Focus periods before the long break</label>
  <span class="unit"><input id="rounds" type="number" min="1" max="12" bind:value={rounds} onchange={save} /></span>
</div>

<div class="field">
  <label class="label" for="presets">Quick timers</label>
  <span class="unit"><input id="presets" type="text" class="presets" bind:value={presets} onchange={save} spellcheck="false" autocomplete="off" /> minutes</span>
  <p class="help">
    The lengths <span class="mono">timer</span> offers, separated by commas (at most 8, each up to 1440). Any other length is typed after the word:
    <span class="mono">timer 17</span>, <span class="mono">timer 1h30</span>, <span class="mono">timer 90s tea</span>.
  </p>
</div>

<div class="field">
  <label class="check"><input type="checkbox" bind:checked={auto} onchange={save} /> Start the next period by itself</label>
  <p class="help">Off, a finished period waits for you to start the next one. Either way a sound plays when a period ends.</p>
</div>

{#if error}<p class="error">{error}</p>{/if}
<p class="ok" aria-live="polite">{saved ? "Saved" : ""}</p>

<style>
  .grid {
    display: grid;
    grid-template-columns: minmax(0, 250px) auto;
    align-items: center;
    gap: 12px 16px;
    max-width: 420px;
    margin-bottom: 22px;
  }
  .unit {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--text-3);
    font-size: 12.5px;
  }
  input[type="number"] {
    width: 72px;
  }
  input.presets {
    width: 180px;
  }
</style>
