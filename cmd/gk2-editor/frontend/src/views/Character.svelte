<script lang="ts">
  import { m } from "../lib/paraglide/messages.js";
  import { app, api, edit, lang, run } from "../lib/app.svelte";
  import NumberInput from "../components/NumberInput.svelte";
  import Money from "../components/Money.svelte";
  import Icon from "../components/Icon.svelte";
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

  const ownLabels: Record<string, () => string> = {
    tech_red: () => m.res_tech_red(),
    tech_green: () => m.res_tech_green(),
    tech_blue: () => m.res_tech_blue(),
    stamina: () => m.res_stamina(),
    happiness: () => m.res_happiness(),
    village_REP: () => m.res_village(),
  };

  function label(r: session.ResourceView): string {
    return r.name || ownLabels[r.type]?.() || r.type;
  }

  function group(g: string): session.ResourceView[] {
    return (player?.resources ?? []).filter((r) => r.group === g);
  }

  const main = $derived(group("main"));
  const reputation = $derived(group("reputation"));
  const perks = $derived(group("perks").filter((r) => r.value > 0));
  const zones = $derived(group("zones").filter((r) => r.name));
  const other = $derived(
    group("other").filter((r) => {
      const q = filter.trim().toLowerCase();
      return !q || r.type.toLowerCase().includes(q);
    }),
  );

  function setRes(r: session.ResourceView, v: number) {
    void edit(() => api.SetResource(r.type, v));
  }
</script>

{#if player}
  <div class="grid">
    <section class="wide">
      <h2>{m.money()}</h2>
      <Money value={player.money} oncommit={(v) => edit(() => api.SetMoney(v))} />
      <p class="muted small">{m.money_limit()}</p>
    </section>

    <section>
      <h2>{m.res_group_main()}</h2>
      <table>
        <tbody>
          {#each main as r (r.type)}
            <tr>
              <td class="icon"><Icon name={r.icon} size={24} /></td>
              <td>{label(r)}</td>
              <td class="num">
                <NumberInput value={r.value} min={0} max={1e6} label={label(r)} oncommit={(v) => setRes(r, v)} />
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
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

    {#if reputation.length}
      <section>
        <h2>{m.res_group_reputation()}</h2>
        <table>
          <tbody>
            {#each reputation as r (r.type)}
              <tr>
                <td>{label(r)}</td>
                <td class="num">
                  <NumberInput value={r.value} min={-100} max={100} label={label(r)} oncommit={(v) => setRes(r, v)} />
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </section>
    {/if}

    {#if zones.length}
      <section>
        <h2>{m.res_group_zones()}</h2>
        <p class="muted small">{m.zones_note()}</p>
        <table>
          <tbody>
            {#each zones as r (r.type)}
              <tr><td>{r.name}</td><td class="num">{r.value}</td></tr>
            {/each}
          </tbody>
        </table>
      </section>
    {/if}

    {#if perks.length}
      <section class="wide">
        <h2>{m.res_group_perks()}</h2>
        <ul class="chips">
          {#each perks as r (r.type)}
            <li>{label(r)}</li>
          {/each}
        </ul>
      </section>
    {/if}

    <details class="wide other">
      <summary><h2>{m.res_group_other()}</h2></summary>
      <p class="muted small">{m.other_note()}</p>
      <input type="search" placeholder={m.filter()} bind:value={filter} />
      <table>
        <tbody>
          {#each other as r (r.type)}
            <tr>
              <td class="id">{r.type}</td>
              <td class="num">
                <NumberInput value={r.value} min={-1e9} max={1e9} step={0.5} label={r.type} oncommit={(v) => setRes(r, v)} />
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </details>
  </div>
{/if}

<style>
  .grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 2rem; align-items: start; }
  .wide { grid-column: 1 / -1; }
  section, .other { display: grid; gap: 0.6rem; }
  .icon { width: 32px; padding-right: 0; }
  td:first-child { white-space: nowrap; }
  td.num :global(input[type="number"]) { width: 6rem; }
  .chips { list-style: none; margin: 0; padding: 0; display: flex; flex-wrap: wrap; gap: 0.4rem; }
  .chips li { padding: 0.15rem 0.6rem; border: 1px solid var(--line); border-radius: 999px; background: var(--slate); font-size: var(--step--1); }
  .other summary { cursor: pointer; }
  .other summary h2 { display: inline; }
  .branch { display: inline-block; width: 0.7rem; height: 0.7rem; border-radius: 50%; margin-right: 0.4rem; background: var(--stone); }
  .branch[data-branch="talent_orange"] { background: #d98b3a; }
  .branch[data-branch="talent_red"] { background: #c8644b; }
  .branch[data-branch="talent_green"] { background: var(--moss); }
  .branch[data-branch="talent_yellow"] { background: var(--candle); }
  .branch[data-branch="talent_blue"] { background: #6f9fc9; }
  @media (max-width: 1100px) { .grid { grid-template-columns: 1fr; } }
</style>
