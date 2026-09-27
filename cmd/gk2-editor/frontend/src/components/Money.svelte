<script lang="ts">
  import { m } from "../lib/paraglide/messages.js";
  import NumberInput from "./NumberInput.svelte";
  import Icon from "./Icon.svelte";

  const maxMoney = 16777216;

  let { value, oncommit }: { value: number; oncommit: (v: number) => void } = $props();

  const total = $derived(Math.round(value));
  const coins = $derived([
    { key: "gld", label: m.gold(), amount: Math.floor(total / 10000), unit: 10000, max: Math.floor(maxMoney / 10000) },
    { key: "slv", label: m.silver(), amount: Math.floor((total % 10000) / 100), unit: 100, max: 99 },
    { key: "brz", label: m.bronze(), amount: total % 100, unit: 1, max: 99 },
  ]);

  function set(unit: number, amount: number) {
    const current = coins.find((c) => c.unit === unit)?.amount ?? 0;
    oncommit(Math.min(maxMoney, total + (amount - current) * unit));
  }
</script>

<div class="coins">
  {#each coins as c (c.key)}
    <label class="coin">
      <Icon name={c.key} size={28} />
      <NumberInput value={c.amount} max={c.max} label={c.label} oncommit={(v) => set(c.unit, v)} />
      <span class="small muted">{c.label}</span>
    </label>
  {/each}
</div>

<style>
  .coins { display: flex; flex-wrap: wrap; gap: 1.25rem; align-items: center; }
  .coin { display: flex; align-items: center; gap: 0.45rem; }
  .coin :global(input[type="number"]) { width: 5.5rem; }
</style>
