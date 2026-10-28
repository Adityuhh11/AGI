import "@fontsource-variable/geist";
import "@fontsource-variable/geist-mono";
import "@fontsource-variable/sora";
import "./theme.css";
import { mount } from "svelte";
import App from "./App.svelte";
import { catchUnexpected } from "./lib/oops";

catchUnexpected();

// Line icons are drawn with a stroke of a whole number of device pixels, the only width a screen draws sharply: at 125% a
// 1.6 px line is a blurred 2 px one. The resize event also fires when the window moves to a screen of another scale.
function setLineWidth() {
  const dpr = window.devicePixelRatio || 1;
  document.documentElement.style.setProperty("--line-width", `${Math.max(1, Math.round(1.6 * dpr)) / dpr}px`);
}
setLineWidth();
window.addEventListener("resize", setLineWidth);

const target = document.getElementById("app")!;
if (location.hash.startsWith("#setup")) {
  import("./Setup.svelte").then(({ default: Setup }) => mount(Setup, { target }));
} else if (location.hash.startsWith("#notes")) {
  import("./Notes.svelte").then(({ default: Notes }) => mount(Notes, { target }));
} else if (location.hash.startsWith("#chat")) {
  import("./Chat.svelte").then(({ default: Chat }) => mount(Chat, { target }));
} else if (location.hash.startsWith("#timer")) {
  import("./Timer.svelte").then(({ default: Timer }) => mount(Timer, { target }));
} else if (location.hash.startsWith("#settings")) {
  // Its own chunk: the search bar's page never loads or runs the settings code.
  import("./Settings.svelte").then(({ default: Settings }) => mount(Settings, { target }));
} else {
  mount(App, { target });
}
