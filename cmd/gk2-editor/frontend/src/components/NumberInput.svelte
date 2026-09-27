<script lang="ts">
  let {
    value,
    min = 0,
    max = Number.MAX_SAFE_INTEGER,
    step = 1,
    label,
    oncommit,
  }: {
    value: number;
    min?: number;
    max?: number;
    step?: number;
    label: string;
    oncommit: (v: number) => void;
  } = $props();

  let draft = $state("");
  let editing = $state(false);

  function begin() {
    draft = String(value);
    editing = true;
  }

  function commit() {
    editing = false;
    const v = Number(draft);
    if (draft.trim() === "" || !Number.isFinite(v) || v === value) return;
    oncommit(Math.min(max, Math.max(min, v)));
  }

  function key(e: KeyboardEvent) {
    if (e.key === "Enter") (e.currentTarget as HTMLInputElement).blur();
    if (e.key === "Escape") {
      editing = false;
      (e.currentTarget as HTMLInputElement).blur();
    }
  }
</script>

<input
  type="number"
  aria-label={label}
  {min}
  {max}
  {step}
  value={editing ? draft : value}
  onfocus={begin}
  oninput={(e) => (draft = e.currentTarget.value)}
  onblur={commit}
  onkeydown={key}
/>

<style>
  input { width: 8.5rem; text-align: right; }
</style>
