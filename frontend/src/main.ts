import "@fontsource-variable/geist";
import "@fontsource-variable/geist-mono";
import "@fontsource-variable/sora";
import "./theme.css";
import { mount } from "svelte";
import App from "./App.svelte";

const target = document.getElementById("app")!;
if (location.hash === "#settings") {
  // Its own chunk: the search bar's page never loads or runs the settings code.
  import("./Settings.svelte").then(({ default: Settings }) => mount(Settings, { target }));
} else {
  mount(App, { target });
}
