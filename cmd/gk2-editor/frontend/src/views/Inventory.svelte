<script lang="ts">
  import { m } from "../lib/paraglide/messages.js";
  import { app, api, edit, lang, run } from "../lib/app.svelte";
  import NumberInput from "../components/NumberInput.svelte";
  import ItemTable from "../components/ItemTable.svelte";
  import ItemFinder from "../components/ItemFinder.svelte";
  import type { gamedata, session } from "../lib/wailsjs/go/models";

  type Where = "bag" | "belt";

  let bag = $state<session.ContainerView | null>(null);
  let belt = $state<session.ContainerView | null>(null);
  let chests = $state<session.StorageView[]>([]);
  let target = $state<string>("bag");
  let addCount = $state(1);
  let replacing = $state<{ where: Where; item: session.ItemView } | null>(null);

  $effect(() => {
    void app.revision;
    const l = lang();
    run(() => api.Container("bag", l)).then((v) => v && (bag = v));
    run(() => api.Container("belt", l)).then((v) => v && (belt = v));
    run(() => api.Storages(l)).then((v) => v && (chests = v));
  });

  const zones = $derived.by(() => {
    const out: { name: string; chests: session.StorageView[] }[] = [];
    for (const c of chests) {
      const name = c.zoneName || m.no_zone();
      const last = out[out.length - 1];
      if (last && last.name === name) last.chests.push(c);
      else out.push({ name, chests: [c] });
    }
    return out;
  });

  function targetFull(t: string): boolean {
    if (t === "bag") return full(bag);
    if (t === "belt") return full(belt);
    const c = chests.find((x) => x.uniqueId === t);
    return !c || c.items.length >= c.capacity;
  }

  $effect(() => {
    if (!bag || !belt || !targetFull(target)) return;
    const free = ["bag", "belt", ...chests.map((c) => c.uniqueId)].find((t) => !targetFull(t));
    if (free) target = free;
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

  function toggle(where: Where, item: session.ItemView) {
    replacing = replacing?.item.uniqueId === item.uniqueId ? null : { where, item };
  }
</script>

<div class="layout">
  <ItemFinder
    heading={replacing ? `${m.replace()}: ${replacing.item.name}` : m.add_item()}
    actionLabel={replacing ? m.replace() : m.add()}
    disabled={!replacing && targetFull(target)}
    onpick={pick}
  >
    {#if !replacing}
      <select bind:value={target} aria-label={m.add_item()}>
        <option value="bag" disabled={full(bag)}>{m.backpack()}</option>
        <option value="belt" disabled={full(belt)}>{m.belt()}</option>
        {#each zones as z (z.name)}
          <optgroup label={z.name}>
            {#each z.chests as c (c.uniqueId)}
              <option value={c.uniqueId} disabled={c.items.length >= c.capacity}>{c.name} ({c.items.length}/{c.capacity})</option>
            {/each}
          </optgroup>
        {/each}
      </select>
      <NumberInput value={addCount} min={1} max={9999} label={m.count()} oncommit={(v) => (addCount = v)} />
    {/if}
  </ItemFinder>

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
        <ItemTable items={c.view.items} container={c.where} replacingId={replacing?.item.uniqueId} onreplace={(it) => toggle(c.where, it)} />
      </section>
    {/if}
  {/each}
</div>

<style>
  .layout { display: grid; gap: 2rem; }
  section { display: grid; gap: 0.5rem; }
  .head { display: flex; align-items: baseline; gap: 1rem; }
  .head button { margin-left: auto; }
  select { flex: 1; }
  .layout :global(.controls input[type="number"]) { width: 5rem; }
</style>
