<script lang="ts">
  import { m } from "../lib/paraglide/messages.js";
  import { app, api, edit, lang, run } from "../lib/app.svelte";
  import NumberInput from "../components/NumberInput.svelte";
  import type { session } from "../lib/wailsjs/go/models";
  import { branchName } from "../lib/branches";

  let player = $state<session.Player | null>(null);
  let filter = $state("");

  $effect(() => {
    void app.revision;
    run(() => api.Player(lang())).then((p) => {
      if (p) player = p;
    });
  });

  const resources = $derived(
    (player?.resources ?? []).filter((r) => {
      const q = filter.trim().toLowerCase();
      return !q || r.type.includes(q) || r.name.toLowerCase().includes(q);
    }),
  );

  const silver = $derived(Math.floor((player?.money ?? 0) / 100));
  const copper = $derived(Math.round((player?.money ?? 0) % 100));
</script>

{#if player}
  <div class="grid">
    <section class="money">
      <h2>{m.money()}</h2>
      <div class="row">
        <NumberInput value={player.money} max={16777216} label={m.money()} oncommit={(v) => edit(() => api.SetMoney(v))} />
        <span class="coins">{m.money_split({ silver, copper })}</span>
      </div>
      <p class="muted small">{m.money_limit()}</p>
    </section>

    <section>
      <h2>{m.talent_points()}</h2>
      <table>
        <thead>
          <tr><th></th><th class="num">{m.level()}</th><th class="num">{m.free_points()}</th></tr>
        </thead>
        <tbody>
          {#each player.talents as t (t.id)}
            <tr>
              <td>
                <span class="branch" data-branch={t.id}></span>
                {branchName(t.id)}
              </td>
              <td class="num">{t.level}</td>
              <td class="num">
                <NumberInput value={t.freePoints} max={9999} label={m.free_points()} oncommit={(v) => edit(() => api.SetTalentPoints(t.id, v))} />
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </section>

    <section class="wide">
      <div class="head">
        <h2>{m.resources()}</h2>
        <input type="search" placeholder={m.filter()} bind:value={filter} />
      </div>
      <table>
        <tbody>
          {#each resources as r (r.type)}
            <tr>
              <td>
                {r.name !== r.type ? r.name : ""}
                <span class="id">{r.type}</span>
              </td>
              <td class="num">
                <NumberInput value={r.value} min={-1e9} max={1e9} step={0.5} label={r.type} oncommit={(v) => edit(() => api.SetResource(r.type, v))} />
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </section>
  </div>
{/if}

<style>
  .grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 2rem; align-items: start; }
  .wide { grid-column: 1 / -1; }
  section { display: grid; gap: 0.6rem; }
  .row { display: flex; align-items: center; gap: 1rem; }
  .coins { font-family: var(--serif); font-size: var(--step-1); color: var(--candle); white-space: nowrap; }
  td:first-child { white-space: nowrap; }
  .head { display: flex; justify-content: space-between; align-items: center; gap: 1rem; }
  .branch { display: inline-block; width: 0.7rem; height: 0.7rem; border-radius: 50%; margin-right: 0.4rem; background: var(--stone); }
  .branch[data-branch="talent_orange"] { background: #d98b3a; }
  .branch[data-branch="talent_red"] { background: #c8644b; }
  .branch[data-branch="talent_green"] { background: var(--moss); }
  .branch[data-branch="talent_yellow"] { background: var(--candle); }
  .branch[data-branch="talent_blue"] { background: #6f9fc9; }
  @media (max-width: 1100px) { .grid { grid-template-columns: 1fr; } }
</style>
