<script lang="ts">
  import { m } from "../lib/paraglide/messages.js";
  import { app, api, edit, lang, run } from "../lib/app.svelte";
  import Icon from "../components/Icon.svelte";
  import type { session } from "../lib/wailsjs/go/models";

  let zombies = $state<session.ZombieView[]>([]);

  $effect(() => {
    void app.revision;
    run(() => api.Zombies(lang())).then((z) => z && (zombies = z));
  });

  const types: Record<number, () => string> = {
    0: m.type_0, 1: m.type_1, 2: m.type_2, 3: m.type_3, 4: m.type_4,
    5: m.type_5, 6: m.type_6, 7: m.type_7, 8: m.type_8,
  };
</script>

<div class="stack">
  <h1>{m.zombies_title()}</h1>
  {#if zombies.length === 0}
    <p class="muted">{m.no_zombies()}</p>
  {/if}
  {#each zombies as z (z.name)}
    <article>
      <header>
        <h2>{z.displayName}</h2>
        <span class="muted">{types[z.type]?.() ?? z.type}</span>
        <span class="muted small">{z.zone}</span>
        <button onclick={() => edit(() => api.MaxZombie(z.name, 999999, 328))}>{m.max_zombie()}</button>
      </header>
      <div class="cols">
        <div>
          <h3>{m.tech_points()}</h3>
          <ul class="tech">
            <li><span class="dot red"></span>{z.techRed}</li>
            <li><span class="dot blue"></span>{z.techBlue}</li>
            <li><span class="dot green"></span>{z.techGreen}</li>
          </ul>
        </div>
        <div>
          <h3>{m.body_parts()}</h3>
          <ul>
            {#each z.parts as p (p.uniqueId)}
              <li><Icon name={p.icon} size={32} /> {p.name}{#if p.count > 1}<span class="muted">{" \u00d7" + p.count}</span>{/if}</li>
            {/each}
          </ul>
        </div>
        <div>
          <h3>{m.perks()}</h3>
          <ul>
            {#each z.perkNames as p, i (i)}
              <li>{p}</li>
            {/each}
          </ul>
        </div>
      </div>
    </article>
  {/each}
</div>

<style>
  .stack { display: grid; gap: 1.5rem; }
  article { border-top: 1px solid var(--line); padding-top: 1rem; display: grid; gap: 0.75rem; }
  header { display: flex; align-items: baseline; gap: 1rem; }
  header button { margin-left: auto; }
  h3 { font-family: var(--sans); font-weight: 400; color: var(--stone); font-size: var(--step--1); margin: 0 0 0.3rem; }
  .cols { display: grid; grid-template-columns: 1fr 1.4fr 1.4fr; gap: 1.5rem; }
  ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 0.2rem; }
  .tech li { display: flex; align-items: center; gap: 0.5rem; font-family: var(--serif); font-size: var(--step-1); }
  .dot { width: 0.65rem; height: 0.65rem; border-radius: 50%; display: inline-block; }
  .dot.red { background: #c8644b; }
  .dot.blue { background: #6f9fc9; }
  .dot.green { background: var(--moss); }
</style>
