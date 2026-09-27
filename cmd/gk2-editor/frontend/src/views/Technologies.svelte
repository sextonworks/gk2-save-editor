<script lang="ts">
  import { m } from "../lib/paraglide/messages.js";
  import { app, api, lang, run } from "../lib/app.svelte";
  import type { session } from "../lib/wailsjs/go/models";

  let techs = $state<session.TechView[]>([]);

  $effect(() => {
    void app.revision;
    run(() => api.Techs(lang())).then((t) => t && (techs = t));
  });

  const groups = $derived([
    { state: "unlocked", title: m.tech_unlocked() },
    { state: "available", title: m.tech_available() },
    { state: "hidden", title: m.tech_hidden() },
  ].map((g) => ({ ...g, items: techs.filter((t) => t.state === g.state) })));
</script>

<div class="stack">
  <h1>{m.technologies_title()}</h1>
  <div class="note">
    <p class="muted">{m.tech_note()}</p>
    <button onclick={() => (app.page = "character")}>{m.go_to_character()}</button>
  </div>
  <div class="cols">
    {#each groups as g (g.state)}
      <section>
        <h2>{g.title} <span class="muted">{g.items.length}</span></h2>
        <ul class={g.state}>
          {#each g.items as t (t.id)}
            <li title={t.id}>{t.name}</li>
          {/each}
        </ul>
      </section>
    {/each}
  </div>
</div>

<style>
  .stack { display: grid; gap: 1rem; }
  .note { display: flex; gap: 1rem; align-items: center; justify-content: space-between; }
  .cols { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 1.5rem; align-items: start; }
  section { display: grid; gap: 0.5rem; }
  ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 0.15rem; }
  li { padding: 0.15rem 0; border-bottom: 1px solid #252d35; }
  ul.available li { color: var(--candle); }
  ul.hidden li { color: var(--stone-dim); }
</style>
