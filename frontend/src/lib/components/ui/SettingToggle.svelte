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

  async function toggleChange(newValue: boolean) {
    savedPending = false;
    try {
      const result = await onUpdate(setting.key, String(newValue));
      if (result.requires_restart) {
        savedPending = true;
      }
    } catch {
      // Error handled by parent
    }
  }
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving} {savedPending}>
  <div class="flex items-center gap-3">
    <button
      class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors {setting.value === 'true' ? 'bg-slate-600' : 'bg-slate-300'} {isSaving ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}"
      onclick={() => toggleChange(setting.value !== 'true')}
      disabled={isSaving}
      role="switch"
      aria-checked={setting.value === 'true'}
      aria-label={setting.label}
      type="button"
    >
      <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {setting.value === 'true' ? 'translate-x-6' : 'translate-x-1'}"></span>
    </button>
    <span class="text-sm text-slate-500">{setting.value === 'true' ? 'Enabled' : 'Disabled'}</span>
  </div>
</SettingRow>
