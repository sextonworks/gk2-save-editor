<script lang="ts">
  import { m } from "../lib/paraglide/messages.js";
  import { app, api, edit, refresh, run } from "../lib/app.svelte";

  async function pick(kind: "save" | "game") {
    const dir = await run(() => api.PickFolder(kind === "save" ? m.save_folder() : m.game_folder()));
    if (!dir) return;
    const env = await run(() => (kind === "save" ? api.UseFolders(dir, "") : api.UseFolders("", dir)));
    if (env) app.env = env;
  }

  async function openSlot(name: string) {
    await edit(() => api.Open(name));
    if (app.state?.open) app.page = "character";
  }

  function when(iso: string): string {
    return iso ? new Date(iso).toLocaleString(app.locale) : "";
  }
</script>

<div class="stack">
  <h1>{m.saves_title()}</h1>

  <dl>
    <dt>{m.save_folder()}</dt>
    <dd>
      <span class="path">{app.env?.saveDir || m.not_found()}</span>
      <button onclick={() => pick("save")}>{m.choose_folder()}</button>
    </dd>
    <dt>{m.game_folder()}</dt>
    <dd>
      <span class="path">{app.env?.gameDir || m.not_found()}</span>
      <button onclick={() => pick("game")}>{m.choose_folder()}</button>
    </dd>
  </dl>

  {#if app.env && (app.env.gameDirError || app.env.catalogError)}
    <p class="muted">{m.game_data_missing()}</p>
  {/if}

  {#if app.env && app.env.slots.length === 0}
    <p class="muted">{m.no_slots()}</p>
  {:else if app.env}
    <table>
      <thead>
        <tr><th></th><th>{m.day_column()}</th><th></th><th></th></tr>
      </thead>
      <tbody>
        {#each app.env.slots as slot (slot.name)}
          <tr>
            <td class="slot">{slot.name}</td>
            <td>{slot.day || ""}</td>
            <td class="muted small">{slot.savedAt || when(slot.modified)}</td>
            <td class="num">
              {#if app.state?.open && app.state.slot === slot.name}
                <span class="muted">{m.is_open()}</span>
              {:else}
                <button onclick={() => openSlot(slot.name)}>{m.open()}</button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
  <div><button class="quiet" onclick={refresh}>{m.reload()}</button></div>
</div>

<style>
  .stack { display: grid; gap: 1.25rem; max-width: 760px; }
  dl { display: grid; grid-template-columns: max-content 1fr; gap: 0.5rem 1rem; margin: 0; align-items: center; }
  dt { color: var(--stone); }
  dd { margin: 0; display: flex; gap: 0.75rem; align-items: center; min-width: 0; }
  .path { overflow-wrap: anywhere; font-size: var(--step--1); flex: 1; }
  .slot { font-family: var(--serif); font-size: var(--step-1); }
</style>
