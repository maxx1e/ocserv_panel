<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<SettingDef>;
    savingKey: string | null;
  };

  let { setting, onUpdate, savingKey }: Props = $props();
  const isSaving = $derived(savingKey === setting.key);
  let savedPending = $state(false);
  let error = $state('');

  async function handleClick() {
    savedPending = false;
    error = '';
    try {
      const result = await onUpdate(setting.key, '');
      if (result.requires_restart) {
        savedPending = true;
      }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to execute';
    }
  }
</script>

<SettingRow
  label={setting.label}
  description={settingDescriptions[setting.key]}
  requiresRestart={setting.requires_restart}
  isSaving={isSaving}
  {savedPending}
>
  <div class="flex items-center gap-3">
    <button
      class="px-4 py-2 bg-slate-600 text-white text-sm font-medium rounded-lg hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      onclick={handleClick}
      disabled={isSaving}
    >
      {setting.label}
    </button>
    {#if error}
      <span class="text-sm text-red-600">{error}</span>
    {/if}
  </div>
</SettingRow>
