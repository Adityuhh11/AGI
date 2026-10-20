<script lang="ts">
  import { onMount } from "svelte";
  import { Service as Palette } from "../../bindings/agi/internal/palette";
  import type { TagInfo } from "../../bindings/agi/internal/palette";
  import ActionPanel from "./ActionPanel.svelte";
  import { newTag, tagLines } from "./tags";

  interface Props {
    /** The tag id of the thing being tagged (`file:...`, `app:...`) and its name. */
    id: string;
    title: string;
    /** Runs a change and shows why it failed, if it does. */
    attempt: (action: () => Promise<unknown>) => Promise<void>;
    /** The thing now carries these tags. */
    onchange: (id: string, tags: string[]) => void;
    onclose: () => void;
  }
  let { id, title, attempt, onchange, onclose }: Props = $props();

  let all = $state<TagInfo[]>([]);
  let applied = $state<string[]>([]);
  let loaded = $state(false);

  onMount(async () => {
    const [tags, mine] = await Promise.all([Palette.Tags(), Palette.TagsOf([id])]);
    all = tags ?? [];
    applied = mine?.[id] ?? [];
    loaded = true;
  });

  /** Puts a tag on the thing, or takes it off. A name typed that is new becomes a tag. */
  async function toggle(line: string) {
    const name = line.startsWith("new:") ? line.slice(4) : line;
    await attempt(async () => {
      applied = (await Palette.Tag(id, name)) ?? [];
      onchange(id, applied);
      all = (await Palette.Tags()) ?? [];
    });
  }
</script>

{#if loaded}
  <ActionPanel
    actions={tagLines(all, applied)}
    onrun={toggle}
    {onclose}
    placeholder={"Tag “" + title + "”"}
    create={(typed: string) => newTag(typed, all)}
  />
{/if}
