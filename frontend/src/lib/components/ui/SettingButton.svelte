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

  async function handleClick() {
    savedPending = false;
    try {
      const result = await onUpdate(setting.key, '');
      if (result.requires_restart) {
        savedPending = true;
      }
    } catch {
      // Error handled by parent
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
  <button
    class="px-4 py-2 bg-slate-600 text-white text-sm font-medium rounded-lg hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
    onclick={handleClick}
    disabled={isSaving}
  >
    {setting.label}
  </button>
</SettingRow>
