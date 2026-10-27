<script lang="ts">
  interface Props {
    name: "code" | "text" | "image" | "video" | "file" | "app" | "web" | "tag" | "pin" | "clip" | "snippet" | "settings" | "calc" | "system" | "note" | "timer";
    size?: number;
  }
  let { name, size = 20 }: Props = $props();

  // One grammar: a 24 grid, a 1.6 stroke, round caps and joins. Files are a page with a folded corner.
  const page = "M7 3h7l4 4v13a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z M14 3v4h4";
  const drawings: Record<Props["name"], string> = {
    code: `${page} M10.5 12.2 9 14l1.5 1.8 M13.5 12.2 15 14l-1.5 1.8`,
    text: `${page} M9 12h6 M9 15h6 M9 18h3.5`,
    image: "M5 5h14a1 1 0 0 1 1 1v12a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1z M4.5 16.5l4.2-4.2 3 3 2.3-2.3 5.5 5.5 M15.5 9.7a.7.7 0 1 0 0 .01",
    video: "M5 5h14a1 1 0 0 1 1 1v12a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1z M10.2 9.2v5.6l4.6-2.8z",
    file: page,
    tag: "M4 12.5V5a1 1 0 0 1 1-1h7.5l7.2 7.2a1 1 0 0 1 0 1.4l-6.6 6.6a1 1 0 0 1-1.4 0z M8.5 8.5h.01",
    web: "M12 4a8 8 0 1 0 0 16 8 8 0 0 0 0-16z M4 12h16 M12 4c2.4 2.2 3.6 4.9 3.6 8s-1.2 5.8-3.6 8c-2.4-2.2-3.6-4.9-3.6-8s1.2-5.8 3.6-8z",
    snippet: "M8 5.5H6.5A1.5 1.5 0 0 0 5 7v10a1.5 1.5 0 0 0 1.5 1.5H8 M16 5.5h1.5A1.5 1.5 0 0 1 19 7v10a1.5 1.5 0 0 1-1.5 1.5H16 M9 9.5h6 M9 12.5h6 M9 15.5h3.5",
    clip: "M9 4h6v2H9z M7 5.5h-.5A1.5 1.5 0 0 0 5 7v12a1.5 1.5 0 0 0 1.5 1.5h11A1.5 1.5 0 0 0 19 19V7a1.5 1.5 0 0 0-1.5-1.5H17 M8.5 11h7 M8.5 14.5h7",
    settings: "M4 8h9 M17 8h3 M4 16h3 M11 16h9 M15 5.5v5 M9 13.5v5",
    pin: "M9 4h6 M10 4l-.6 5.6L7 12v1.5h10V12l-2.4-2.4L14 4 M12 13.5V20",
    calc: "M7 3.5h10a1 1 0 0 1 1 1v15a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1v-15a1 1 0 0 1 1-1z M8.5 6.5h7v3h-7z M9 13h.01 M12 13h.01 M15 13h.01 M9 16.5h.01 M12 16.5h.01 M15 16.5h.01",
    system: "M12 4v8 M7.2 6.8a7 7 0 1 0 9.6 0",
    note: "M6 4h12a1 1 0 0 1 1 1v10l-4 4H6a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1z M14.5 19v-3.5a.5.5 0 0 1 .5-.5h4 M8.5 8.5h7 M8.5 12h4",
    timer: "M12 7a7 7 0 1 0 0 14 7 7 0 0 0 0-14z M12 11v4l2.4 1.4 M10 3.5h4 M12 3.5V7",
    app: "M5 5h5.5v5.5H5z M13.5 5H19v5.5h-5.5z M5 13.5h5.5V19H5z M13.5 13.5H19V19h-5.5z",
  };
</script>

<svg
  class="line-icon"
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  <path d={drawings[name]} />
</svg>
