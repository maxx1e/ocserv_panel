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

  async function toggleChange(newValue: boolean) {
    if (disabled) return;
    savedPending = false;
    savedSuccess = false;
    error = '';
    try {
      const result = await onUpdate(setting.key, String(newValue));
      if (result.requires_restart) {
        savedPending = true;
      } else {
        savedSuccess = true;
        if (successTimer) clearTimeout(successTimer);
        successTimer = setTimeout(() => { savedSuccess = false; }, 2000);
      }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    }
  }
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving} {savedPending} {savedSuccess}>
  <div class="flex items-center gap-3">
    <button
      class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors {setting.value === 'true' ? 'bg-slate-600' : 'bg-slate-300'} {isSaving || disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}"
      onclick={() => toggleChange(setting.value !== 'true')}
      disabled={isSaving || disabled}
      role="switch"
      aria-checked={setting.value === 'true'}
      aria-label={setting.label}
      type="button"
    >
      <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {setting.value === 'true' ? 'translate-x-6' : 'translate-x-1'}"></span>
    </button>
    <span class="text-sm text-slate-500">{setting.value === 'true' ? 'Enabled' : 'Disabled'}</span>
    {#if error}
      <span class="text-sm text-red-600 max-w-md break-words">{error}</span>
    {/if}
  </div>
</SettingRow>
