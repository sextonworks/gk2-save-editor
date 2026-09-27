<script lang="ts">
  import { m } from "../lib/paraglide/messages.js";
  import { app, api, edit, lang, run } from "../lib/app.svelte";
  import NumberInput from "../components/NumberInput.svelte";
  import ItemTable from "../components/ItemTable.svelte";
  import ItemFinder from "../components/ItemFinder.svelte";
  import Icon from "../components/Icon.svelte";
  import type { gamedata, session } from "../lib/wailsjs/go/models";

  const previewCount = 8;

  let chests = $state<session.StorageView[]>([]);
  let loaded = $state(false);
  let open = $state<Record<string, boolean>>({});
  let target = $state<session.StorageView | null>(null);
  let replacing = $state<{ chest: string; item: session.ItemView } | null>(null);
  let addCount = $state(1);

  $effect(() => {
    void app.revision;
    run(() => api.Storages(lang())).then((v) => {
      if (!v) return;
      chests = v;
      loaded = true;
      if (target) target = v.find((c) => c.uniqueId === target?.uniqueId) ?? null;
    });
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

  const titles = $derived.by(() => {
    const out: Record<string, string> = {};
    for (const z of zones) {
      const seen: Record<string, number> = {};
      const total: Record<string, number> = {};
      for (const c of z.chests) total[c.name] = (total[c.name] ?? 0) + 1;
      for (const c of z.chests) {
        seen[c.name] = (seen[c.name] ?? 0) + 1;
        out[c.uniqueId] = total[c.name] > 1 ? `${c.name} ${seen[c.name]}` : c.name;
      }
    }
    return out;
  });

  const full = $derived(!!target && target.items.length >= target.capacity);

  function pick(entry: gamedata.Entry) {
    if (replacing) {
      const r = replacing;
      replacing = null;
      void edit(() => api.SwapItem(r.chest, r.item.uniqueId, entry.id));
      return;
    }
    if (target) {
      const id = target.uniqueId;
      void edit(() => api.AddItem(id, entry.id, addCount));
    }
  }

  function startAdd(c: session.StorageView) {
    replacing = null;
    target = c;
    open[c.uniqueId] = true;
  }

  function startReplace(c: session.StorageView, item: session.ItemView) {
    replacing = replacing?.item.uniqueId === item.uniqueId ? null : { chest: c.uniqueId, item };
    target = replacing ? c : null;
  }

  function close() {
    target = null;
    replacing = null;
  }
</script>

<div class="page">
  <p class="muted">{m.storage_note()}</p>

  {#if target}
    <div class="finder">
      <ItemFinder
        heading={replacing ? `${m.replace()}: ${replacing.item.name}` : m.add_to({ name: titles[target.uniqueId] ?? target.name })}
        actionLabel={replacing ? m.replace() : m.add()}
        disabled={!replacing && full}
        onpick={pick}
      >
        {#if !replacing}
          <NumberInput value={addCount} min={1} max={9999} label={m.count()} oncommit={(v) => (addCount = v)} />
          <span class="muted small">{m.slots_used({ used: target.items.length, total: target.capacity })}</span>
        {/if}
        <button class="quiet close" onclick={close}>{m.close()}</button>
      </ItemFinder>
    </div>
  {/if}

  {#if loaded && chests.length === 0}
    <p class="muted">{m.storage_none()}</p>
  {/if}

  {#each zones as z (z.name)}
    <section>
      <h2>{z.name}</h2>
      {#each z.chests as c (c.uniqueId)}
        <details class="chest" class:active={target?.uniqueId === c.uniqueId} bind:open={open[c.uniqueId]}>
          <summary>
            <span class="chev" aria-hidden="true">▸</span>
            <span class="title">{titles[c.uniqueId]}</span>
            <span class="preview">
              {#each c.items.slice(0, previewCount) as it (it.uniqueId)}
                <Icon name={it.icon} size={32} alt={it.name} />
              {/each}
            </span>
            <span class="fill" title={m.slots_used({ used: c.items.length, total: c.capacity })}>
              <span class="bar"><span style="width: {Math.min(100, (100 * c.items.length) / Math.max(1, c.capacity))}%"></span></span>
              <span class="small muted">{c.items.length}/{c.capacity}</span>
            </span>
          </summary>
          <div class="body">
            {#if c.items.length === 0}
              <p class="muted small">{m.chest_empty()}</p>
            {:else}
              <ItemTable items={c.items} container={c.uniqueId} replacingId={replacing?.item.uniqueId} onreplace={(it) => startReplace(c, it)} />
            {/if}
            <button onclick={() => startAdd(c)} disabled={c.items.length >= c.capacity}>{m.add_item()}</button>
          </div>
        </details>
      {/each}
    </section>
  {/each}
</div>

<style>
  .page { display: grid; gap: 1.5rem; }
  section { display: grid; gap: 0.5rem; }
  .finder {
    position: sticky; top: 0; z-index: 1;
    background: var(--night); padding: 0.75rem; border: 1px solid var(--line); border-left: 3px solid var(--candle);
  }
  .close { margin-left: auto; }
  .chest { border: 1px solid var(--line); border-radius: 4px; background: var(--slate); }
  .chest.active { border-color: var(--candle); }
  summary {
    display: grid; grid-template-columns: auto minmax(8rem, auto) minmax(0, 1fr) auto; align-items: center; gap: 0.75rem;
    padding: 0.5rem 0.75rem; cursor: pointer; list-style: none;
  }
  summary::-webkit-details-marker { display: none; }
  .chev { color: var(--stone); transition: transform 0.15s; }
  .chest[open] .chev { transform: rotate(90deg); }
  .title { font-family: var(--serif); font-size: var(--step-1); }
  @media (prefers-reduced-motion: reduce) { .chev { transition: none; } }
  .preview { display: flex; gap: 0; overflow: hidden; height: 32px; }
  .fill .small { min-width: 3.2rem; text-align: right; }
  .fill { display: flex; align-items: center; gap: 0.5rem; }
  .bar { width: 5rem; height: 6px; background: #1a2026; border-radius: 3px; overflow: hidden; }
  .bar span { display: block; height: 100%; background: var(--moss); }
  .body { display: grid; gap: 0.6rem; padding: 0 0.75rem 0.75rem; justify-items: start; }
  .body :global(table) { width: 100%; }
</style>
