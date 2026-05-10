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

  // Parse validation constraints from setting.validation
  const constraints = $derived(() => {
    let min: number | null = null;
    let max: number | null = null;
    if (!setting.validation) return { min, max };
    for (const part of setting.validation.split(',')) {
      const trimmed = part.trim();
      if (trimmed.startsWith('min:')) {
        min = parseInt(trimmed.slice(4), 10);
      } else if (trimmed.startsWith('max:')) {
        max = parseInt(trimmed.slice(4), 10);
      }
    }
    return { min, max };
  });

  const constraintHint = $derived(() => {
    const { min, max } = constraints();
    if (min !== null && max !== null) return `${min} – ${max}`;
    if (min !== null) return `min: ${min}`;
    if (max !== null) return `max: ${max}`;
    return '';
  });

  function validateLocal(val: string): string {
    const n = parseInt(val, 10);
    if (isNaN(n)) return 'Must be a number';
    if (!setting.validation) return '';
    for (const part of setting.validation.split(',')) {
      const trimmed = part.trim();
      if (trimmed.startsWith('min:')) {
        const min = parseInt(trimmed.slice(4), 10);
        if (n < min) return `Must be at least ${min}`;
      } else if (trimmed.startsWith('max:')) {
        const max = parseInt(trimmed.slice(4), 10);
        if (n > max) return `Must be at most ${max}`;
      }
    }
    return '';
  }

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

    const validationError = validateLocal(editValue);
    if (validationError) {
      error = validationError;
      return;
    }

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
    <div class="flex flex-col gap-1">
      <div class="flex items-center gap-2">
        <input
          type="number"
          bind:value={editValue}
          onkeydown={handleKeydown}
          class="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 bg-white focus:border-slate-500 focus:ring-1 focus:ring-slate-500 w-32"
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
      </div>
      {#if constraintHint()}
        <span class="text-xs text-slate-400">{constraintHint()}</span>
      {/if}
    </div>
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
