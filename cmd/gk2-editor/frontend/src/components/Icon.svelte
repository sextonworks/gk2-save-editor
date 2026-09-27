<script lang="ts">
  import { app } from "../lib/app.svelte";

  let { name, size = 32, alt = "" }: { name: string; size?: number; alt?: string } = $props();
  let failed = $state(false);

  const src = $derived(name ? `/icons/${encodeURIComponent(name)}.png?r=${app.state?.iconsReady ? 1 : 0}` : "");

  $effect(() => {
    void src;
    failed = false;
  });
</script>

{#if src && !failed}
  <img {src} {alt} width={size} height={size} loading="lazy" onerror={() => (failed = true)} />
{:else}
  <span class="blank" style="width: {size}px; height: {size}px;" aria-hidden="true"></span>
{/if}

<style>
  img { image-rendering: pixelated; object-fit: contain; flex: none; vertical-align: middle; }
  .blank { display: inline-block; flex: none; border: 1px dashed #3a4550; border-radius: 3px; vertical-align: middle; }
</style>
