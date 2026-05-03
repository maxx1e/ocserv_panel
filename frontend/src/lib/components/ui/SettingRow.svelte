<script lang="ts">
  import SettingTooltip from './SettingTooltip.svelte';

  type Props = {
    label: string;
    description?: string;
    requiresRestart?: boolean;
    isSaving?: boolean;
    children: import('svelte').Snippet;
  };

  let { label, description, requiresRestart = false, isSaving = false, children }: Props = $props();
</script>

<div class="grid grid-cols-1 md:grid-cols-[240px_1fr] gap-1 md:gap-4 py-3 border-b border-slate-100 last:border-0 items-center">
  <div class="flex items-center justify-end md:text-right">
    <span class="font-medium text-slate-700 text-sm">{label}</span>
    <SettingTooltip {description} />
    {#if requiresRestart}
      <span
        class="inline-flex items-center justify-center w-5 h-5 rounded-full bg-amber-100 text-amber-600 text-xs font-bold ml-1"
        title="Requires service restart"
      >&#x21BB;</span>
    {/if}
    {#if isSaving}
      <span class="text-xs text-slate-400 ml-2">Saving…</span>
    {/if}
  </div>
  <div class="flex items-center gap-2">
    {@render children()}
  </div>
</div>
