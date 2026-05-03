<script lang="ts">
  import type { SettingDef } from '$lib/api';
  import SettingToggle from './SettingToggle.svelte';

  interface Props {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<void>;
    savingKey: string | null;
  }

  let { setting, onUpdate, savingKey }: Props = $props();

  let editValue = $state('');
  let showPassword = $state(false);
  let editing = $state(false);
  let error = $state('');

  const isSaving = $derived(savingKey === setting.key);
  const inputType = $derived(
    setting.type === 'password' && showPassword ? 'text' : setting.type
  );

  // For multi-checkbox — sync when setting prop changes
  let checkboxValues = $state<string[]>([]);

  $effect(() => {
    checkboxValues = setting.value && setting.type === 'multi-checkbox'
      ? setting.value.split(',')
      : [];
  });

  const checkboxChanged = $derived(
    checkboxValues.join(',') !== (setting.value || '')
  );

  function startEdit() {
    editValue = setting.value;
    editing = true;
    error = '';
  }

  async function save() {
    error = '';
    try {
      await onUpdate(setting.key, editValue);
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

  async function toggleChange(newValue: boolean) {
    error = '';
    try {
      await onUpdate(setting.key, String(newValue));
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    }
  }

  function toggleCheckbox(option: string) {
    if (checkboxValues.includes(option)) {
      checkboxValues = checkboxValues.filter((v) => v !== option);
    } else {
      checkboxValues = [...checkboxValues, option];
    }
  }

  async function saveCheckboxes() {
    error = '';
    try {
      await onUpdate(setting.key, checkboxValues.join(','));
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    }
  }

  function cancelCheckboxes() {
    checkboxValues = setting.value ? setting.value.split(',') : [];
    error = '';
  }
</script>

<div class="py-3 first:pt-0 border-b border-slate-100 last:border-0">
  <div class="flex items-center gap-2 mb-2">
    <span class="font-medium text-slate-700 text-sm">{setting.label}</span>
    {#if setting.requires_restart}
      <span class="inline-flex items-center justify-center w-5 h-5 rounded-full bg-amber-100 text-amber-600 text-xs font-bold" title="Requires service restart">&#x21BB;</span>
    {/if}
    {#if isSaving}
      <span class="text-xs text-slate-400">Saving…</span>
    {/if}
  </div>

  {#if setting.type === 'toggle'}
    <div class="flex items-center gap-3">
      <SettingToggle
        checked={setting.value === 'true'}
        onchange={toggleChange}
        disabled={isSaving}
        label={setting.label}
      />
      <span class="text-sm text-slate-500">{setting.value === 'true' ? 'Enabled' : 'Disabled'}</span>
    </div>
  {:else if setting.type === 'readonly'}
    <div class="flex items-center gap-2">
      <span class="text-sm text-slate-600">{setting.value || '—'}</span>
    </div>
  {:else if setting.type === 'button'}
    <div class="flex items-center gap-2">
      <button
        class="px-4 py-2 bg-slate-600 text-white text-sm font-medium rounded-lg hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        onclick={() => onUpdate(setting.key, '')}
        disabled={isSaving}
      >
        {setting.label}
      </button>
    </div>
  {:else if setting.type === 'select'}
    {#if editing}
      <div class="flex items-center gap-2">
        <select
          bind:value={editValue}
          class="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 bg-white focus:border-slate-500 focus:ring-1 focus:ring-slate-500"
        >
          {#each (setting.options ?? []) as option}
            <option value={option}>{option}</option>
          {/each}
        </select>
        <div class="flex items-center gap-2">
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
      </div>
    {:else}
      <div class="flex items-center gap-2">
        <span class="text-sm text-slate-600">{setting.value || '—'}</span>
        <button
          class="px-3 py-1.5 text-sm font-medium text-slate-600 border border-slate-300 rounded-lg hover:bg-slate-50 transition-colors"
          onclick={startEdit}
        >Edit</button>
      </div>
    {/if}
  {:else if setting.type === 'multi-checkbox'}
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
    </div>
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
  {:else if setting.type === 'password'}
    {#if editing}
      <div class="flex items-center gap-2">
        <input
          type={showPassword ? 'text' : 'password'}
          bind:value={editValue}
          class="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 bg-white focus:border-slate-500 focus:ring-1 focus:ring-slate-500"
        />
        <button
          class="px-3 py-2 text-sm font-medium text-slate-600 border border-slate-300 rounded-lg hover:bg-slate-50 transition-colors"
          onclick={() => (showPassword = !showPassword)}
          type="button"
        >
          {showPassword ? 'Hide' : 'Show'}
        </button>
        <div class="flex items-center gap-2">
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
      </div>
    {:else}
      <div class="flex items-center gap-2">
        <span class="text-sm text-slate-500">••••••••</span>
        <button
          class="px-3 py-1.5 text-sm font-medium text-slate-600 border border-slate-300 rounded-lg hover:bg-slate-50 transition-colors"
          onclick={startEdit}
        >Edit</button>
      </div>
    {/if}
  {:else}
    <!-- text / number -->
    {#if editing}
      <div class="flex items-center gap-2">
        <input
          type={setting.type}
          bind:value={editValue}
          class="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 bg-white focus:border-slate-500 focus:ring-1 focus:ring-slate-500"
        />
        <div class="flex items-center gap-2">
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
      </div>
    {:else}
      <div class="flex items-center gap-2">
        <span class="text-sm text-slate-600">{setting.value || '—'}</span>
        <button
          class="px-3 py-1.5 text-sm font-medium text-slate-600 border border-slate-300 rounded-lg hover:bg-slate-50 transition-colors"
          onclick={startEdit}
        >Edit</button>
      </div>
    {/if}
  {/if}

  {#if error}
    <span class="block mt-1 text-sm text-red-600">{error}</span>
  {/if}
</div>
