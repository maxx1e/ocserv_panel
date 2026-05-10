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

  let checkboxValues = $state<string[]>([]);

  // Sync checkbox state when parent prop changes (e.g. after save or reload).
  $effect(() => {
    checkboxValues = setting.value ? setting.value.split(',') : [];
  });
  let error = $state('');
  let savedPending = $state(false);
  let savedSuccess = $state(false);
  let successTimer: ReturnType<typeof setTimeout> | null = null;
  const isSaving = $derived(savingKey === setting.key);
  const checkboxChanged = $derived(checkboxValues.join(',') !== (setting.value || ''));

  function toggleCheckbox(option: string) {
    if (disabled) return;
    if (checkboxValues.includes(option)) {
      checkboxValues = checkboxValues.filter((v) => v !== option);
    } else {
      checkboxValues = [...checkboxValues, option];
    }
  }

  async function saveCheckboxes() {
    savedPending = false;
    savedSuccess = false;
    error = '';
    try {
      const result = await onUpdate(setting.key, checkboxValues.join(','));
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

  function cancelCheckboxes() {
    checkboxValues = setting.value ? setting.value.split(',') : [];
    error = '';
  }
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving} {savedPending} {savedSuccess}>
  <div class="flex flex-col gap-2">
    {#each (setting.options ?? []) as option}
      <label class="flex items-center gap-2 text-sm text-slate-700 {disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}">
        <input
          type="checkbox"
          checked={checkboxValues.includes(option)}
          onchange={() => toggleCheckbox(option)}
          {disabled}
          class="rounded border-slate-300 text-slate-600 focus:ring-slate-500 disabled:opacity-50"
        />
        {option}
      </label>
    {/each}
    {#if checkboxChanged && !disabled}
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
    <span class="text-sm text-red-600 max-w-md break-words">{error}</span>
  {/if}
</SettingRow>
