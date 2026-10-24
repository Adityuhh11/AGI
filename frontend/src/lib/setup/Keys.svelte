<script lang="ts">
  import { Service as Settings } from "../../../bindings/agi/internal/settingsui";
  import type { View } from "../../../bindings/agi/internal/settingsui";
  import HotkeyField from "../settings/HotkeyField.svelte";

  let { view }: { view: View } = $props();

  // svelte-ignore state_referenced_locally
  let startup = $state(view.Autostart);
  let error = $state("");

  // The field shows its own refusal ("that hotkey is taken"); this only saves what the person picked.
  const saveHotkey = (accelerator: string) => Settings.SaveGeneral(accelerator, view.Browser, view.SearchURL);

  async function saveStartup() {
    error = await Settings.SetAutostart(startup);
    if (error !== "") startup = !startup;
  }

  const KEYS: [string, string][] = [
    ["vsc  Enter", "opens an app (one that is already open is brought forward)"],
    ["?  or  web:", "searches the web in your browser"],
    ["gh rust", "a quicklink: that search on GitHub"],
    ["clip:", "what you copied; Enter pastes it"],
    ["snip:", "your snippets; Enter pastes one"],
    ["settings", "opens Settings, or one of its pages"],
    ["Tab", "switches between everything, apps, files and the web"],
    ["Ctrl+K", "what can be done with the selected row"],
    ["Esc", "closes the bar"],
  ];
</script>

<h2>Open it from anywhere</h2>
<p class="lede">Press the hotkey in any program, then type what you want.</p>

<div class="field">
  <span class="label" id="setup-hotkey-label">Your hotkey</span>
  <HotkeyField labelId="setup-hotkey-label" value={view.Hotkey} defaultValue={view.DefaultHotkey} onsave={saveHotkey} />
  <p class="help">Click it to record another. AGI says if another program already uses it.</p>
</div>

<div class="field">
  <label class="check"><input type="checkbox" bind:checked={startup} onchange={saveStartup} /> Start when I sign in</label>
  {#if error}<p class="error">{error}</p>{/if}
</div>

<div class="field">
  <span class="label">In the bar</span>
  <dl>
    {#each KEYS as [typed, does] (typed)}
      <dt><kbd>{typed}</kbd></dt>
      <dd>{does}</dd>
    {/each}
  </dl>
</div>

<style>
  dl {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 8px 16px;
    margin: 0;
    align-items: baseline;
  }
  dt {
    white-space: pre;
  }
  dd {
    margin: 0;
    color: var(--text-2);
    font-size: 13px;
  }
</style>
