<script lang="ts">
  interface Props {
    name: "code" | "text" | "image" | "file" | "app" | "web" | "tag";
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
    tag: "M4 12.5V5a1 1 0 0 1 1-1h7.5l7.2 7.2a1 1 0 0 1 0 1.4l-6.6 6.6a1 1 0 0 1-1.4 0z M8.5 8.5h.01",
    web: "M12 4a8 8 0 1 0 0 16 8 8 0 0 0 0-16z M4 12h16 M12 4c2.4 2.2 3.6 4.9 3.6 8s-1.2 5.8-3.6 8c-2.4-2.2-3.6-4.9-3.6-8s1.2-5.8 3.6-8z",
    app: "M5 5h5.5v5.5H5z M13.5 5H19v5.5h-5.5z M5 13.5h5.5V19H5z M13.5 13.5H19V19h-5.5z",
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
