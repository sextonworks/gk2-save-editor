<script lang="ts">
  import { m } from "../lib/paraglide/messages.js";
  import { api, edit } from "../lib/app.svelte";
  import NumberInput from "./NumberInput.svelte";
  import Icon from "./Icon.svelte";
  import type { session } from "../lib/wailsjs/go/models";

  let {
    items,
    container,
    replacingId = "",
    onreplace,
  }: {
    items: session.ItemView[];
    container: string;
    replacingId?: string;
    onreplace: (item: session.ItemView) => void;
  } = $props();
</script>

<table>
  <tbody>
    {#each items as it (it.uniqueId)}
      <tr class:selected={replacingId === it.uniqueId}>
        <td class="icon"><Icon name={it.icon} size={40} /></td>
        <td>
          <span class="name">{it.name}</span>
          {#if it.name !== it.id}<span class="id">{it.id}</span>{/if}
        </td>
        <td class="num stack small muted">{it.stack > 1 ? m.stack({ n: it.stack }) : ""}</td>
        <td class="num">
          <NumberInput value={it.count} min={1} max={9999} label={m.count()} oncommit={(v) => edit(() => api.SetItemCount(container, it.uniqueId, v))} />
        </td>
        <td class="num actions">
          <button class="quiet" onclick={() => onreplace(it)}>{m.replace()}</button>
          <button class="quiet danger" onclick={() => edit(() => api.RemoveItem(container, it.uniqueId))}>{m.remove()}</button>
        </td>
      </tr>
    {/each}
  </tbody>
</table>

<style>
  .icon { width: 48px; padding: 0 0 0 0.25rem; }
  .name { margin-right: 0.5rem; }
  .stack { white-space: nowrap; }
  .actions { white-space: nowrap; width: 1%; }
  td.num :global(input[type="number"]) { width: 5.5rem; }
  tr.selected td { background: #2a2a20; }
</style>
