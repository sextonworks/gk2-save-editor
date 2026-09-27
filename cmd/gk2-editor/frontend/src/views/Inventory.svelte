<script lang="ts">
  import { untrack } from "svelte";
  import { get } from "svelte/store";
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import { m } from "../lib/paraglide/messages.js";
  import { app, api, edit, lang, run } from "../lib/app.svelte";
  import NumberInput from "../components/NumberInput.svelte";
  import type { gamedata, session } from "../lib/wailsjs/go/models";

  type Where = "bag" | "belt";

  let bag = $state<session.ContainerView | null>(null);
  let belt = $state<session.ContainerView | null>(null);
  let query = $state("");
  let allIds = $state(false);
  let results = $state<gamedata.Entry[]>([]);
  let target = $state<Where>("bag");
  let addCount = $state(1);
  let replacing = $state<{ where: Where; item: session.ItemView } | null>(null);
  let searchError = $state("");
  let scroller = $state<HTMLDivElement | null>(null);

  $effect(() => {
    void app.revision;
    const l = lang();
    run(() => api.Container("bag", l)).then((v) => v && (bag = v));
    run(() => api.Container("belt", l)).then((v) => v && (belt = v));
  });

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
    estimateSize: () => 40,
    overscan: 10,
  });

  $effect(() => {
    const count = results.length;
    const el = scroller;
    untrack(() => get(virtualizer).setOptions({ count, getScrollElement: () => el }));
  });

  function pick(entry: gamedata.Entry) {
    if (replacing) {
      const r = replacing;
      replacing = null;
      void edit(() => api.SwapItem(r.where, r.item.uniqueId, entry.id));
      return;
    }
    void edit(() => api.AddItem(target, entry.id, addCount));
  }

  function full(view: session.ContainerView | null): boolean {
    return !!view && view.items.length >= view.capacity;
  }
</script>

<div class="layout">
  <div class="containers">
    {#each [{ where: "bag" as Where, view: bag, title: m.backpack() }, { where: "belt" as Where, view: belt, title: m.belt() }] as c (c.where)}
      {#if c.view}
        <section>
          <div class="head">
            <h2>{c.title}</h2>
            <span class="muted small">{m.slots_used({ used: c.view.items.length, total: c.view.capacity })}</span>
            {#if c.where === "belt"}
              <button onclick={() => edit(() => api.EquipBest())}>{m.equip_best()}</button>
            {/if}
          </div>
          <table>
            <tbody>
              {#each c.view.items as it (it.uniqueId)}
                <tr class:selected={replacing?.item.uniqueId === it.uniqueId}>
                  <td>
                    <span class="name">{it.name}</span>
                    {#if it.name !== it.id}<span class="id">{it.id}</span>{/if}
                  </td>
                  <td class="num">
                    <NumberInput value={it.count} min={1} max={9999} label={m.count()} oncommit={(v) => edit(() => api.SetItemCount(c.where, it.uniqueId, v))} />
                  </td>
                  <td class="num actions">
                    <button class="quiet" onclick={() => (replacing = replacing?.item.uniqueId === it.uniqueId ? null : { where: c.where, item: it })}>
                      {m.replace()}
                    </button>
                    <button class="quiet danger" onclick={() => edit(() => api.RemoveItem(c.where, it.uniqueId))}>{m.remove()}</button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </section>
      {/if}
    {/each}
  </div>

  <aside class="finder">
    <h2>{replacing ? `${m.replace()}: ${replacing.item.name}` : m.add_item()}</h2>
    <input type="search" placeholder={m.search_items()} bind:value={query} />
    <label class="small muted check"><input type="checkbox" bind:checked={allIds} /> {m.all_ids()}</label>
    {#if !replacing}
      <div class="target">
        <select bind:value={target} aria-label={m.add_item()}>
          <option value="bag" disabled={full(bag)}>{m.backpack()}</option>
          <option value="belt" disabled={full(belt)}>{m.belt()}</option>
        </select>
        <NumberInput value={addCount} min={1} max={9999} label={m.count()} oncommit={(v) => (addCount = v)} />
      </div>
    {/if}
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
              <span class="rname">
                {e.name}
                {#if e.name !== e.id}<span class="id">{e.id}</span>{/if}
              </span>
              <button onclick={() => pick(e)} disabled={!replacing && full(target === "bag" ? bag : belt)}>
                {replacing ? m.replace() : m.add()}
              </button>
            </div>
          {/if}
        {/each}
      </div>
    </div>
  </aside>
</div>

<style>
  .layout { display: grid; gap: 2rem; }
  .containers { display: grid; gap: 2rem; }
  section { display: grid; gap: 0.5rem; }
  .head { display: flex; align-items: baseline; gap: 1rem; }
  .head button { margin-left: auto; }
  .name { margin-right: 0.5rem; }
  .actions { white-space: nowrap; width: 1%; }
  td.num :global(input[type="number"]) { width: 5.5rem; }
  tr.selected td { background: #2a2a20; }
  .finder { order: -1; display: grid; gap: 0.6rem; grid-template-columns: minmax(0, 1fr) auto; align-items: center; }
  .finder h2, .finder .results, .finder p { grid-column: 1 / -1; }
  .finder :global(input[type="number"]) { width: 5rem; }
  .check { display: flex; gap: 0.4rem; align-items: center; }
  .target { display: flex; gap: 0.5rem; }
  .target select { flex: 1; }
  .results { height: 240px; overflow: auto; border: 1px solid var(--line); border-radius: 4px; }
  .result {
    position: absolute; top: 0; left: 0; right: 0;
    display: flex; align-items: center; justify-content: space-between; gap: 0.5rem;
    padding: 0 0.5rem; border-bottom: 1px solid #252d35;
  }
  .rname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .rname .id { margin-left: 0.35rem; }
</style>
