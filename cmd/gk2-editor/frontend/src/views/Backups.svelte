<script lang="ts">
  import { m } from "../lib/paraglide/messages.js";
  import { app, api, edit, run } from "../lib/app.svelte";
  import type { session } from "../lib/wailsjs/go/models";

  let list = $state<session.BackupView[]>([]);
  let confirming = $state("");

  $effect(() => {
    void app.revision;
    run(() => api.Backups()).then((b) => b && (list = b));
  });

  async function restore(name: string) {
    confirming = "";
    await edit(() => api.Restore(name));
    app.notice = m.restored({ name });
  }
</script>

<div class="stack">
  <h1>{m.backups_title()}</h1>
  <p class="muted small">{app.env?.backupDir}</p>
  {#if list.length === 0}
    <p class="muted">{m.backups_empty()}</p>
  {:else}
    <table>
      <tbody>
        {#each list as b (b.name)}
          <tr>
            <td>{new Date(b.created).toLocaleString(app.locale)}</td>
            <td class="muted small">{b.files.join(", ")}{#if b.legacy}. {m.legacy_backup()}{/if}</td>
            <td class="num">
              {#if confirming === b.name}
                <span class="confirm">
                  {m.restore_confirm()}
                  <button class="primary" onclick={() => restore(b.name)}>{m.confirm()}</button>
                  <button class="quiet" onclick={() => (confirming = "")}>{m.cancel()}</button>
                </span>
              {:else}
                <button disabled={app.state?.gameRunning} onclick={() => (confirming = b.name)}>{m.restore()}</button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

<style>
  .stack { display: grid; gap: 0.75rem; max-width: 900px; }
  .confirm { display: inline-flex; gap: 0.5rem; align-items: center; text-align: left; }
</style>
