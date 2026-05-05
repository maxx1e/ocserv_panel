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

  let checkboxValues = $state<string[]>(setting.value ? setting.value.split(',') : []);
  let error = $state('');
  let savedPending = $state(false);
  const isSaving = $derived(savingKey === setting.key);
  const checkboxChanged = $derived(checkboxValues.join(',') !== (setting.value || ''));

  function toggleCheckbox(option: string) {
    if (checkboxValues.includes(option)) {
      checkboxValues = checkboxValues.filter((v) => v !== option);
    } else {
      checkboxValues = [...checkboxValues, option];
    }
  }

  async function saveCheckboxes() {
    savedPending = false;
    error = '';
    try {
      const result = await onUpdate(setting.key, checkboxValues.join(','));
      if (result.requires_restart) {
        savedPending = true;
      }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    }
  }

  function cancelCheckboxes() {
    checkboxValues = setting.value ? setting.value.split(',') : [];
    error = '';
  }
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving} {savedPending}>
  <div class="flex flex-col gap-2">
    {#each (setting.options ?? []) as option}
      <label class="flex items-center gap-2 text-sm text-slate-700 cursor-pointer">
        <input
          type="checkbox"
          checked={checkboxValues.includes(option)}
          onchange={() => toggleCheckbox(option)}
          class="rounded border-slate-300 text-slate-600 focus:ring-slate-500"
        />
        {option}
      </label>
    {/each}
    {#if checkboxChanged}
      <div class="flex items-center gap-2 mt-2">
        <button
          class="px-3 py-2 bg-slate-600 text-white text-sm font-medium rounded-lg hover:bg-slate-700 disabled:opacity-50 transition-colors"
          onclick={saveCheckboxes}
          disabled={isSaving}
        >Save</button>
        <button
          class="px-3 py-2 border border-slate-300 text-slate-600 text-sm font-medium rounded-lg hover:bg-slate-50 transition-colors"
          onclick={cancelCheckboxes}
        >Cancel</button>
      </div>
    {/if}
  </div>
  {#if error}
    <span class="text-sm text-red-600">{error}</span>
  {/if}
</SettingRow>
