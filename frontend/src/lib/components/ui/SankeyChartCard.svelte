<script lang="ts">
  import SankeyChart from './SankeyChart.svelte';
  import { formatBytes } from '$lib/monitoring';

  type InterfaceEntry = { name: string; bytes: number };

  type Props = {
    interfaces: InterfaceEntry[];
    mode: 'rate' | 'cumulative';
    window?: string;
    height?: number;
  };

  let { interfaces, mode, window: w, height = 180 }: Props = $props();

  const VIOLET_SOLID = 'rgba(139,92,246,0.85)';
  const VIOLET_PILL_BG = 'rgba(139,92,246,0.12)';

  const total = $derived(interfaces.reduce((s, i) => s + i.bytes, 0));

  const totalLabel = $derived(
    total === 0
      ? '—'
      : mode === 'rate'
        ? `${formatBytes(total, 'iec')}/s total`
        : `${formatBytes(total, 'iec')} window`
  );
</script>

<div class="bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden">
  <!-- Header -->
  <div
    class="bg-slate-50 px-4 py-2.5 flex items-center justify-between"
    style="border-left: 4px solid {VIOLET_SOLID}"
  >
    <span class="text-[11px] font-bold uppercase tracking-wider text-slate-500">VPN Traffic Share</span>
    <span
      class="text-[11px] font-bold px-2.5 py-0.5 rounded-full"
      style="color: {VIOLET_SOLID}; background: {VIOLET_PILL_BG}"
    >
      {totalLabel}
    </span>
  </div>

  <!-- Chart -->
  <div class="px-2 pb-1 pt-1">
    <SankeyChart {interfaces} {mode} {height} />
  </div>

  <!-- Footer -->
  <div class="px-4 pb-3">
    {#if w}
      <p class="text-[10px] text-slate-400">Last {w}</p>
    {/if}
  </div>
</div>
