<script lang="ts">
  import { onMount } from "svelte";
  import { ShaderMount, getShaderColorFromString } from "@paper-design/shaders";

  import type { Metal } from "./palettes";
  import { ringShader } from "./ringShader";

  interface Props {
    /** The flow speed to settle at; the ring eases there instead of jumping. */
    speed: number;
    /** False while the panel is closed: the ring stops, and with it every animation frame it costs. */
    running: boolean;
    /** What the metal is made of; it can change while the ring runs. */
    metal: Metal;
  }
  let { speed, running, metal }: Props = $props();

  // A change of speed takes about this long to settle. Typing changes the speed many times a second,
  // and a ring that lurches with every keystroke looks broken.
  const EASE_MS = 350;

  // The shader fills the whole card; the page's own plate covers all but the outer rim, so what shows
  // is a ring of flowing metal.

  const RADIUS = 30; // the card's corner radius in px (--radius in theme.css)

  const reduceMotion = matchMedia("(prefers-reduced-motion: reduce)").matches;

  let host: HTMLDivElement;
  let mount: ShaderMount | undefined;

  onMount(() => {
    try {
      mount = new ShaderMount(
        host,
        ringShader,
        {
          u_colorBack: getShaderColorFromString(metal.back),
          u_colorTint: getShaderColorFromString(metal.tint),
          u_radius: RADIUS,
        },
        undefined,
        0, // stopped until the ring is told it is running
        0,
        // The ring is a few pixels wide: one pixel per CSS pixel is plenty, and the GPU is shared with
        // the embedding model.
        1,
        700_000,
      );
    } catch (error) {
      // No WebGL2: the card's static chrome gradient stands in for the ring.
      console.warn("liquid ring unavailable:", error);
    }
    return () => mount?.dispose();
  });

  // Eases the shader's speed toward `speed` while running, one step per frame, and stops asking for
  // frames once it has arrived. Closing the panel stops the shader at once (nobody sees it stop).
  let current = 0;
  let frame = 0;
  $effect(() => {
    const target = running && !reduceMotion ? speed : 0;
    cancelAnimationFrame(frame);
    if (!mount) return;
    if (target === 0) {
      current = 0;
      mount.setSpeed(0);
      return;
    }
    let last = performance.now();
    const step = (now: number) => {
      current += (target - current) * (1 - Math.exp(-(now - last) / EASE_MS));
      last = now;
      if (Math.abs(target - current) < 0.01) current = target;
      mount?.setSpeed(current);
      if (current !== target) frame = requestAnimationFrame(step);
    };
    frame = requestAnimationFrame(step);
    return () => cancelAnimationFrame(frame);
  });
  // The colours are read before `mount` is looked at: `mount?.setUniforms(...)` would skip reading them
  // while there is no mount yet, and an effect that read nothing never runs again.
  $effect(() => {
    const back = getShaderColorFromString(metal.back);
    const tint = getShaderColorFromString(metal.tint);
    mount?.setUniforms({ u_colorBack: back, u_colorTint: tint });
  });
</script>

<div class="ring" bind:this={host} aria-hidden="true"></div>

<style>
  .ring {
    position: absolute;
    inset: 0;
    border-radius: inherit;
    overflow: hidden;
  }
  .ring :global(canvas) {
    display: block;
    width: 100%;
    height: 100%;
  }
</style>
