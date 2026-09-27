<script lang="ts">
  import { onMount } from "svelte";
  import { m } from "./lib/paraglide/messages.js";
  import { describe } from "./lib/changes";
  import { app, api, changeLocale, edit, listen, refresh, run, type Locale, type Page } from "./lib/app.svelte";
  import Saves from "./views/Saves.svelte";
  import Character from "./views/Character.svelte";
  import Inventory from "./views/Inventory.svelte";
  import Zombies from "./views/Zombies.svelte";
  import Inspirations from "./views/Inspirations.svelte";
  import Technologies from "./views/Technologies.svelte";
  import Inspector from "./views/Inspector.svelte";
  import Backups from "./views/Backups.svelte";

  const pages: { id: Page; label: () => string; needsSave: boolean }[] = [
    { id: "saves", label: () => m.nav_saves(), needsSave: false },
    { id: "character", label: () => m.nav_character(), needsSave: true },
    { id: "inventory", label: () => m.nav_inventory(), needsSave: true },
    { id: "zombies", label: () => m.nav_zombies(), needsSave: true },
    { id: "inspirations", label: () => m.nav_inspirations(), needsSave: true },
    { id: "technologies", label: () => m.nav_technologies(), needsSave: true },
    { id: "inspector", label: () => m.nav_inspector(), needsSave: true },
    { id: "backups", label: () => m.nav_backups(), needsSave: false },
  ];

  const locales = [
    { id: "en", label: "English" },
    { id: "ru", label: "Русский" },
    { id: "zh-CN", label: "简体中文" },
  ] as const;

  onMount(() => {
    listen();
    void refresh();
  });

  let saving = $state(false);

  async function save() {
    saving = true;
    const res = await run(() => api.Write());
    saving = false;
    if (res) {
      app.state = res.state;
      app.notice = res.backup ? m.saved({ path: res.backup }) : "";
      app.revision++;
    }
  }

  const open = $derived(app.state?.open ?? false);
  const blocked = $derived(app.state?.gameRunning || app.state?.conflict);
</script>

<div class="shell">
  <nav>
    <div class="brand">
      <img src="/icon.png" alt="" width="40" height="40" />
      <span>{m.app_title()}</span>
    </div>
    <ul>
      {#each pages as p (p.id)}
        <li>
          <button
            class:current={app.page === p.id}
            disabled={p.needsSave && !open}
            onclick={() => (app.page = p.id)}
          >
            {p.label()}
          </button>
        </li>
      {/each}
    </ul>
    <label class="lang">
      <span class="muted small">{m.language()}</span>
      <select value={app.locale} onchange={(e) => changeLocale(e.currentTarget.value as Locale)}>
        {#each locales as l (l.id)}
          <option value={l.id}>{l.label}</option>
        {/each}
      </select>
    </label>
  </nav>

  <main>
    <header>
      {#if open && app.state}
        <div class="title">
          <h1>{app.state.slot}</h1>
          <span class="day">{m.day({ day: app.state.info.day })}</span>
        </div>
      {/if}
      {#if app.state?.gameRunning}
        <p class="banner candle" role="status">{m.game_running()}</p>
      {/if}
      {#if app.state?.conflict}
        <p class="banner rust" role="alert">
          {m.conflict()}
          <button onclick={() => edit(() => api.Reload())}>{m.reload()}</button>
        </p>
      {/if}
      {#if app.error}
        <p class="banner rust" role="alert">{app.error}</p>
      {/if}
      {#if app.notice}
        <p class="banner moss" role="status">{app.notice}</p>
      {/if}
    </header>

    <section class="page">
        {#if app.page === "saves"}
          <Saves />
        {:else if app.page === "backups"}
          <Backups />
        {:else if !open}
          <div class="empty">
            <h2>{m.no_save_title()}</h2>
            <p class="muted">{m.no_save_body()}</p>
          </div>
        {:else if app.page === "character"}
          <Character />
        {:else if app.page === "inventory"}
          <Inventory />
        {:else if app.page === "zombies"}
          <Zombies />
        {:else if app.page === "inspirations"}
          <Inspirations />
        {:else if app.page === "technologies"}
          <Technologies />
        {:else if app.page === "inspector"}
          <Inspector />
        {/if}
    </section>
  </main>

  <aside class="journal" aria-label={m.journal_title()}>
    <h2>{m.journal_title()}</h2>
    {#if open && app.state && app.state.changes.length > 0}
      <ol>
        {#each app.state.changes as change, i (i)}
          <li>{describe(change)}</li>
        {/each}
      </ol>
      <p class="muted small">{m.journal_hint()}</p>
    {:else}
      <p class="muted">{m.journal_empty()}</p>
    {/if}
    <div class="actions">
      <div class="row">
        <button disabled={!app.state?.canUndo || app.busy} onclick={() => edit(() => api.Undo())}>{m.undo()}</button>
        <button disabled={!app.state?.canRedo || app.busy} onclick={() => edit(() => api.Redo())}>{m.redo()}</button>
      </div>
      <button class="quiet" disabled={!app.state?.dirty || app.busy} onclick={() => edit(() => api.Reload())}>
        {m.discard()}
      </button>
      <button class="primary save" disabled={!app.state?.dirty || blocked || saving} onclick={save}>
        {saving ? m.saving() : m.save()}
      </button>
    </div>
  </aside>
</div>

<style>
  .shell {
    display: grid;
    grid-template-columns: 208px minmax(0, 1fr) 272px;
    height: 100%;
  }

  nav {
    background: var(--slate);
    border-right: 1px solid var(--line);
    display: flex;
    flex-direction: column;
    padding: 1rem 0.75rem;
    gap: 1rem;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    font-family: var(--serif);
    font-weight: 700;
    font-size: var(--step-1);
    line-height: 1.1;
    padding: 0 0.25rem;
  }

  nav ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 2px; flex: 1; align-content: start; }

  nav ul button {
    width: 100%;
    text-align: left;
    background: transparent;
    border: 0;
    border-left: 3px solid transparent;
    border-radius: 0;
    padding: 0.45rem 0.75rem;
    color: var(--stone);
    font-family: var(--serif);
    font-size: var(--step-1);
  }
  nav ul button:hover:not(:disabled) { color: var(--bone); }
  nav ul button.current { color: var(--bone); border-left-color: var(--candle); background: #252e37; }

  .lang { display: grid; gap: 0.25rem; }

  main { display: flex; flex-direction: column; min-width: 0; }

  header { padding: 1rem 1.5rem 0.5rem; display: grid; gap: 0.5rem; }
  .title { display: flex; align-items: baseline; gap: 1rem; }
  .day { font-family: var(--serif); font-size: var(--step-2); color: var(--candle); }

  .banner {
    max-width: none;
    padding: 0.45rem 0.75rem;
    border-left: 3px solid;
    background: var(--slate);
    display: flex;
    align-items: center;
    gap: 1rem;
    justify-content: space-between;
  }
  .banner.candle { border-color: var(--candle); }
  .banner.rust { border-color: var(--rust); }
  .banner.moss { border-color: var(--moss); overflow-wrap: anywhere; }

  .page { flex: 1; overflow: auto; padding: 0.75rem 1.5rem 2rem; }

  .empty { display: grid; gap: 0.5rem; padding-top: 3rem; }

  .journal {
    background: #1a2026;
    border-left: 1px solid var(--line);
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    min-height: 0;
  }
  .journal ol {
    margin: 0;
    padding-left: 1.4rem;
    overflow: auto;
    flex: 1;
    display: grid;
    gap: 0.3rem;
    align-content: start;
  }
  .journal li { font-size: var(--step--1); overflow-wrap: anywhere; }
  .journal li::marker { color: var(--candle); font-family: var(--serif); }
  .journal > p { flex: 1; }
  .actions { display: grid; gap: 0.5rem; }
  .actions .row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.5rem; }
  .save { padding: 0.55rem; font-size: var(--step-1); font-family: var(--serif); }
</style>
