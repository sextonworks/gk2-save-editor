<script lang="ts">
  import type { Snippet } from "svelte";
  import { untrack } from "svelte";
  import { get } from "svelte/store";
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import { m } from "../lib/paraglide/messages.js";
  import { api, lang } from "../lib/app.svelte";
  import Icon from "./Icon.svelte";
  import type { gamedata } from "../lib/wailsjs/go/models";

  let {
    heading,
    actionLabel,
    disabled = false,
    onpick,
    children,
  }: {
    heading: string;
    actionLabel: string;
    disabled?: boolean;
    onpick: (entry: gamedata.Entry) => void;
    children?: Snippet;
  } = $props();

  let query = $state("");
  let allIds = $state(false);
  let results = $state<gamedata.Entry[]>([]);
  let searchError = $state("");
  let scroller = $state<HTMLDivElement | null>(null);

  $effect(() => {
    const q = query;
    const all = allIds;
    const l = lang();
    const timer = setTimeout(async () => {
      try {
        results = await api.SearchItems(q, l, all);
        searchError = "";
      } catch (err) {
        results = [];
        searchError = String(err);
      }
    }, 150);
    return () => clearTimeout(timer);
  });

  const virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: 0,
    getScrollElement: () => scroller,
    estimateSize: () => 48,
    overscan: 10,
  });

  $effect(() => {
    const count = results.length;
    const el = scroller;
    untrack(() => get(virtualizer).setOptions({ count, getScrollElement: () => el }));
  });
</script>

<div class="finder">
  <h2>{heading}</h2>
  <div class="search">
    <input type="search" placeholder={m.search_items()} bind:value={query} />
    <label class="small muted check"><input type="checkbox" bind:checked={allIds} /> {m.all_ids()}</label>
  </div>
  {#if children}<div class="controls">{@render children()}</div>{/if}
  {#if searchError}
    <p class="muted small">{m.game_data_missing()}</p>
  {:else if results.length === 0}
    <p class="muted small">{m.nothing_found()}</p>
  {/if}
  <div class="results" bind:this={scroller}>
    <div style="height: {$virtualizer.getTotalSize()}px; position: relative;">
      {#each $virtualizer.getVirtualItems() as row (row.index)}
        {@const e = results[row.index]}
        {#if e}
          <div class="result" style="transform: translateY({row.start}px); height: {row.size}px;">
            <Icon name={e.icon} size={40} />
            <span class="rname">
              {e.name}
              {#if e.name !== e.id}<span class="id">{e.id}</span>{/if}
            </span>
            <button onclick={() => onpick(e)} {disabled}>{actionLabel}</button>
          </div>
        {/if}
      {/each}
    </div>
  </div>
</div>

<style>
  .finder { display: grid; gap: 0.6rem; }
  .search { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 0.75rem; align-items: center; }
  .controls { display: flex; gap: 0.5rem; align-items: center; }
  .check { display: flex; gap: 0.4rem; align-items: center; white-space: nowrap; }
  .results { height: 264px; overflow: auto; border: 1px solid var(--line); border-radius: 4px; }
  .result {
    position: absolute; top: 0; left: 0; right: 0;
    display: flex; align-items: center; gap: 0.6rem;
    padding: 0 0.5rem; border-bottom: 1px solid #252d35;
  }
  .rname { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .rname .id { margin-left: 0.35rem; }
</style>
