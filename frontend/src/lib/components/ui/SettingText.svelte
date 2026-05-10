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

  let editValue = $state('');
  let editing = $state(false);
  let error = $state('');
  let savedPending = $state(false);
  let savedSuccess = $state(false);
  let successTimer: ReturnType<typeof setTimeout> | null = null;
  const isSaving = $derived(savingKey === setting.key);

  function startEdit() {
    if (disabled) return;
    editValue = setting.value;
    editing = true;
    error = '';
  }

  async function save() {
    savedPending = false;
    savedSuccess = false;
    error = '';
    try {
      const result = await onUpdate(setting.key, editValue);
      if (result.requires_restart) {
        savedPending = true;
      } else {
        savedSuccess = true;
        if (successTimer) clearTimeout(successTimer);
        successTimer = setTimeout(() => { savedSuccess = false; }, 2000);
      }
      editing = false;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    }
  }

  function cancel() {
    editValue = setting.value;
    editing = false;
    error = '';
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') save();
    if (e.key === 'Escape') cancel();
  }
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving} {savedPending} {savedSuccess}>
  {#if editing}
    <input
      type="text"
      bind:value={editValue}
      onkeydown={handleKeydown}
      class="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 bg-white focus:border-slate-500 focus:ring-1 focus:ring-slate-500 w-full max-w-md"
    />
    <button
      class="px-3 py-2 bg-slate-600 text-white text-sm font-medium rounded-lg hover:bg-slate-700 disabled:opacity-50 transition-colors"
      onclick={save}
      disabled={isSaving}
    >Save</button>
    <button
      class="px-3 py-2 border border-slate-300 text-slate-600 text-sm font-medium rounded-lg hover:bg-slate-50 transition-colors"
      onclick={cancel}
    >Cancel</button>
  {:else}
    <span class="text-sm text-slate-600">{setting.value || '—'}</span>
    <button
      class="px-3 py-1.5 text-sm font-medium text-slate-600 border border-slate-300 rounded-lg hover:bg-slate-50 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
      onclick={startEdit}
      {disabled}
    >Edit</button>
  {/if}
  {#if error}
    <span class="text-sm text-red-600 max-w-md break-words">{error}</span>
  {/if}
</SettingRow>
