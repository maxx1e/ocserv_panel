<script lang="ts">
  type Props = {
    value: string;
    windows: string[];
    onchange: (window: string) => void;
  };

  let { value, windows, onchange }: Props = $props();
</script>

<div class="inline-flex rounded-lg bg-slate-100 p-0.5" role="group" aria-label="Monitoring window">
  {#each windows as w}
    <button
      type="button"
      class="px-3 py-1.5 text-sm font-medium rounded-md transition-colors {w === value ? 'bg-white text-slate-700 shadow-sm' : 'text-slate-500 hover:text-slate-700'}"
      aria-pressed={w === value}
      onclick={() => { if (w !== value) onchange(w); }}
    >
      {w}
    </button>
  {/each}
</div>
