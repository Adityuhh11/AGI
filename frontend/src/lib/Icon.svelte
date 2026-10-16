<script lang="ts">
  interface Props {
    name: "code" | "text" | "image" | "file";
    size?: number;
  }
  let { name, size = 20 }: Props = $props();

  // One grammar: a 24 grid, a 1.6 stroke, round caps and joins. Files are a page with a folded corner.
  const page = "M7 3h7l4 4v13a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z M14 3v4h4";
  const drawings: Record<Props["name"], string> = {
    code: `${page} M10.5 12.2 9 14l1.5 1.8 M13.5 12.2 15 14l-1.5 1.8`,
    text: `${page} M9 12h6 M9 15h6 M9 18h3.5`,
    image: "M5 5h14a1 1 0 0 1 1 1v12a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1z M4.5 16.5l4.2-4.2 3 3 2.3-2.3 5.5 5.5 M15.5 9.7a.7.7 0 1 0 0 .01",
    file: page,
  };
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="1.6"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  <path d={drawings[name]} />
</svg>
