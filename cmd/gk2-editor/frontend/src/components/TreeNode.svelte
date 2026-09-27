<script lang="ts">
  import { api, run } from "../lib/app.svelte";
  import { m } from "../lib/paraglide/messages.js";
  import type { session } from "../lib/wailsjs/go/models";
  import TreeNode from "./TreeNode.svelte";

  const page = 200;
  let { node }: { node: session.NodeView } = $props();
  let open = $state(false);
  let kids = $state<session.NodeView[]>([]);

  async function toggle() {
    open = !open;
    if (open && kids.length === 0) await more();
  }

  async function more() {
    const next = await run(() => api.InspectChildren(node.offset, kids.length, page));
    if (next) kids = [...kids, ...next];
  }
</script>

<li>
  <div class="line">
    {#if node.children > 0}
      <button class="toggle" aria-expanded={open} onclick={toggle}>{open ? "−" : "+"}</button>
    {:else}
      <span class="toggle"></span>
    {/if}
    <span class="name">{node.name}</span>
    {#if node.kind === "value"}
      <span class="value">{node.value}</span>
    {:else if node.kind === "array" || node.kind === "bytes"}
      <span class="muted">[{node.value}]</span>
    {/if}
    {#if node.type}<span class="type">{node.type.split(",")[0]}</span>{/if}
  </div>
  {#if open}
    <ul>
      {#each kids as k (k.offset)}
        <TreeNode node={k} />
      {/each}
      {#if kids.length < node.children}
        <li><button class="quiet" onclick={more}>{m.show_more()} ({kids.length} / {node.children})</button></li>
      {/if}
    </ul>
  {/if}
</li>

<style>
  li { list-style: none; }
  ul { margin: 0; padding-left: 1.1rem; border-left: 1px solid #2a333c; }
  .line { display: flex; align-items: baseline; gap: 0.5rem; padding: 0.1rem 0; min-width: 0; }
  .toggle { width: 1.3rem; height: 1.3rem; padding: 0; line-height: 1; flex: none; display: inline-block; text-align: center; }
  .name { color: var(--bone); }
  .value { color: var(--candle); overflow-wrap: anywhere; }
  .type { color: var(--stone-dim); font-size: var(--step--1); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
</style>
