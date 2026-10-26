import "@fontsource-variable/geist";
import "@fontsource-variable/geist-mono";
import "@fontsource-variable/sora";
import "./theme.css";
import { mount } from "svelte";
import App from "./App.svelte";
import { catchUnexpected } from "./lib/oops";

catchUnexpected();
const target = document.getElementById("app")!;
if (location.hash.startsWith("#setup")) {
  import("./Setup.svelte").then(({ default: Setup }) => mount(Setup, { target }));
} else if (location.hash.startsWith("#notes")) {
  import("./Notes.svelte").then(({ default: Notes }) => mount(Notes, { target }));
} else if (location.hash.startsWith("#timer")) {
  import("./Timer.svelte").then(({ default: Timer }) => mount(Timer, { target }));
} else if (location.hash.startsWith("#settings")) {
  // Its own chunk: the search bar's page never loads or runs the settings code.
  import("./Settings.svelte").then(({ default: Settings }) => mount(Settings, { target }));
} else {
  mount(App, { target });
}
