<script lang="ts">
  import { m } from "../lib/paraglide/messages.js";
  import { app, api, run } from "../lib/app.svelte";
  import TreeNode from "../components/TreeNode.svelte";
  import type { session } from "../lib/wailsjs/go/models";

  let roots = $state<session.NodeView[]>([]);
  let query = $state("");
  let hits = $state<session.NodeView[]>([]);

  $effect(() => {
    void app.revision;
    run(() => api.InspectChildren(-1, 0, 50)).then((r) => r && (roots = r));
  });

  $effect(() => {
    const q = query;
    void app.revision;
    const timer = setTimeout(async () => {
      hits = q.trim() ? ((await run(() => api.InspectSearch(q))) ?? []) : [];
    }, 250);
    return () => clearTimeout(timer);
  });
</script>

<div class="stack">
  <h1>{m.inspector_title()}</h1>
  <p class="muted">{m.inspector_note()}</p>
  <input type="search" placeholder={m.inspector_search()} bind:value={query} />
  {#if query.trim()}
    {#if hits.length === 0}
      <p class="muted">{m.nothing_found()}</p>
    {/if}
    <table>
      <tbody>
        {#each hits as h (h.offset)}
          <tr>
            <td class="path">{h.path}</td>
            <td class="value">{h.value}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {:else}
    <ul class="tree">
      {#each roots as r (r.offset)}
        <TreeNode node={r} />
      {/each}
    </ul>
  {/if}
</div>

<style>
  .stack { display: grid; gap: 0.75rem; }
  .tree { margin: 0; padding: 0; font-size: var(--step--1); }
  .path { color: var(--stone); font-size: var(--step--1); overflow-wrap: anywhere; }
  .value { color: var(--candle); overflow-wrap: anywhere; }
</style>
