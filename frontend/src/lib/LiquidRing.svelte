<script lang="ts">
  import { onMount } from "svelte";
  import {
    ShaderMount,
    ShaderFitOptions,
    defaultObjectSizing,
    getShaderColorFromString,
    liquidMetalFragmentShader,
  } from "@paper-design/shaders";

  interface Props {
    /** Flow speed; 0 stops the animation (and its animation frames) entirely. */
    speed: number;
  }
  let { speed }: Props = $props();

  // Liquid chrome: graphite base, white highlights. The shader fills the whole card; the page's own
  // surface covers all but the outer rim, so what shows is a ring of flowing metal.
  const BACK = "#8b8e98";
  const TINT = "#ffffff";

  const reduceMotion = matchMedia("(prefers-reduced-motion: reduce)").matches;

  let host: HTMLDivElement;
  let mount: ShaderMount | undefined;

  onMount(() => {
    try {
      mount = new ShaderMount(
        host,
        liquidMetalFragmentShader,
        {
          u_colorBack: getShaderColorFromString(BACK),
          u_colorTint: getShaderColorFromString(TINT),
          u_repetition: 4,
          u_softness: 0.55,
          u_shiftRed: 0.08,
          u_shiftBlue: 0.08,
          u_distortion: 0.15,
          u_contour: 0.75,
          u_angle: 70,
          u_shape: 0,
          u_isImage: false,
          ...sizing(),
        },
        undefined,
        reduceMotion ? 0 : speed,
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

  $effect(() => mount?.setSpeed(reduceMotion ? 0 : speed));

  function sizing() {
    const s = { ...defaultObjectSizing, fit: "none" as const };
    return {
      u_fit: ShaderFitOptions[s.fit],
      u_scale: s.scale,
      u_rotation: s.rotation,
      u_originX: s.originX,
      u_originY: s.originY,
      u_offsetX: s.offsetX,
      u_offsetY: s.offsetY,
      u_worldWidth: s.worldWidth,
      u_worldHeight: s.worldHeight,
    };
  }
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
