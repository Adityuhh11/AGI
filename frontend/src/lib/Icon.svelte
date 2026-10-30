<script lang="ts">
  interface Props {
    name: "code" | "text" | "image" | "video" | "audio" | "file" | "app" | "web" | "tag" | "pin" | "clip" | "snippet" | "settings" | "calc" | "system" | "note" | "timer" | "chat" | "plus" | "history" | "close" | "brain" | "send" | "stop" | "link" | "search" | "spark" | "info";
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
    audio: "M4 10v4 M8 7v10 M12 4v16 M16 8v8 M20 11v2",
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
    chat: "M5 5.5h14a1 1 0 0 1 1 1V15a1 1 0 0 1-1 1h-7l-4.5 3.5V16H5a1 1 0 0 1-1-1V6.5a1 1 0 0 1 1-1z M8.5 10h7",
    plus: "M12 5v14 M5 12h14",
    history: "M4 12a8 8 0 1 0 2.5-5.8 M4 5v3.6h3.6 M12 8v4.2l2.8 1.8",
    close: "M6.5 6.5l11 11 M17.5 6.5l-11 11",
    brain: "M12 5.2v13.6 M9.6 4.4A2.9 2.9 0 0 0 6.9 7a2.9 2.9 0 0 0-1.7 2.6 2.8 2.8 0 0 0 .8 2.1 2.9 2.9 0 0 0 .5 4.7A2.8 2.8 0 0 0 9.5 19.6 2.6 2.6 0 0 0 12 18.2 M14.4 4.4A2.9 2.9 0 0 1 17.1 7a2.9 2.9 0 0 1 1.7 2.6 2.8 2.8 0 0 1-.8 2.1 2.9 2.9 0 0 1-.5 4.7 2.8 2.8 0 0 1-3.5 3.2A2.6 2.6 0 0 1 12 18.2 M8.5 10.2h1.7 M13.8 13.6h1.7",
    send: "M12 19V6 M6.5 11.5 12 6l5.5 5.5",
    stop: "M8 8h8v8H8z",
    timer: "M12 7a7 7 0 1 0 0 14 7 7 0 0 0 0-14z M12 11v4l2.4 1.4 M10 3.5h4 M12 3.5V7",
    link: "M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1 M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1",
    search: "M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14z M16 16l4 4",
    spark: "M12 4c.6 4.4 2.6 6.4 7 7-4.4.6-6.4 2.6-7 7-.6-4.4-2.6-6.4-7-7 4.4-.6 6.4-2.6 7-7z",
    info: "M12 4a8 8 0 1 0 0 16 8 8 0 0 0 0-16z M12 11v5 M12 8h.01",
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
