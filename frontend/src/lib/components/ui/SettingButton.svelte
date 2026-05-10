<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<SettingDef>;
    savingKey: string | null;
    disabled?: boolean;
  };

  let { setting, onUpdate, savingKey, disabled = false }: Props = $props();
  const isSaving = $derived(savingKey === setting.key);
  let savedPending = $state(false);
  let savedSuccess = $state(false);
  let successTimer: ReturnType<typeof setTimeout> | null = null;
  let error = $state('');

  async function handleClick() {
    if (disabled) return;
    savedPending = false;
    savedSuccess = false;
    error = '';
    try {
      const result = await onUpdate(setting.key, '');
      if (result.requires_restart) {
        savedPending = true;
      } else {
        savedSuccess = true;
        if (successTimer) clearTimeout(successTimer);
        successTimer = setTimeout(() => { savedSuccess = false; }, 2000);
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
  {savedSuccess}
>
  <div class="flex items-center gap-3">
    <button
      class="px-4 py-2 bg-slate-600 text-white text-sm font-medium rounded-lg hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      onclick={handleClick}
      disabled={isSaving || disabled}
    >
      {setting.label}
    </button>
    {#if error}
      <span class="text-sm text-red-600 max-w-md break-words">{error}</span>
    {/if}
  </div>
</SettingRow>
