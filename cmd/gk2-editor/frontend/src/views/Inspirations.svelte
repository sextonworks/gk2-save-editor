<script lang="ts">
  import { m } from "../lib/paraglide/messages.js";
  import { app, api, edit, lang, run } from "../lib/app.svelte";
  import type { session } from "../lib/wailsjs/go/models";
  import { branchName } from "../lib/branches";

  let list = $state<session.InspirationView[]>([]);

  $effect(() => {
    void app.revision;
    run(() => api.Inspirations(lang())).then((l) => l && (list = l));
  });

  const groups = $derived.by(() => {
    const out = new Map<string, session.InspirationView[]>();
    for (const i of list) {
      out.set(i.talent, [...(out.get(i.talent) ?? []), i]);
    }
    return [...out.entries()];
  });

  function pct(i: session.InspirationView): number {
    return i.goal > 0 ? Math.min(100, Math.round((i.current / i.goal) * 100)) : 100;
  }
</script>

<div class="stack">
  <h1>{m.inspirations_title()}</h1>
  <p class="muted">{m.inspirations_note()}</p>
  {#each groups as [talent, items] (talent)}
    <section>
      <div class="head">
        <h2><span class="branch" data-branch={talent}></span>{branchName(talent)}</h2>
        <button disabled={!items.some((i) => i.canRaise)} onclick={() => edit(() => api.Inspire(talent, ""))}>{m.bring_branch()}</button>
      </div>
      <table>
        <tbody>
          {#each items as i (i.id)}
            <tr>
              <td>
                <div>{i.name}</div>
                {#if i.description}<div class="muted small desc">{i.description}</div>{/if}
              </td>
              <td class="bar">
                <div class="track"><div class="fill" class:done={i.ready} style="width: {pct(i)}%"></div></div>
              </td>
              <td class="num">{i.current} / {i.goal}</td>
              <td class="num">
                {#if i.ready}
                  <span class="ready">{m.ready()}</span>
                {:else if i.canRaise}
                  <button class="quiet" onclick={() => edit(() => api.Inspire(i.talent, i.id))}>{m.bring_to_goal()}</button>
                {:else}
                  <span class="muted">&mdash;</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </section>
  {/each}
</div>

<style>
  .stack { display: grid; gap: 1.25rem; }
  section { display: grid; gap: 0.4rem; }
  .head { display: flex; align-items: center; justify-content: space-between; }
  .branch { display: inline-block; width: 0.75rem; height: 0.75rem; border-radius: 50%; margin-right: 0.5rem; background: var(--stone); }
  .branch[data-branch="talent_orange"] { background: #d98b3a; }
  .branch[data-branch="talent_red"] { background: #c8644b; }
  .branch[data-branch="talent_green"] { background: var(--moss); }
  .branch[data-branch="talent_yellow"] { background: var(--candle); }
  .branch[data-branch="talent_blue"] { background: #6f9fc9; }
  .desc { max-width: 48ch; }
  .bar { width: 28%; }
  .track { height: 6px; background: var(--slate-2); border-radius: 3px; overflow: hidden; }
  .fill { height: 100%; background: var(--stone); }
  .fill.done { background: var(--moss); }
  .ready { color: var(--moss); white-space: nowrap; }
  td.num { white-space: nowrap; }
</style>
