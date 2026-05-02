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

<div class="setting-field">
  <div class="field-header">
    <span class="field-label">{setting.label}</span>
    {#if setting.requires_restart}
      <span class="restart-badge" title="Requires service restart">&#x21BB;</span>
    {/if}
    {#if isSaving}
      <span class="saving">Saving…</span>
    {/if}
  </div>

  {#if setting.type === 'toggle'}
    <div class="field-row">
      <SettingToggle
        checked={setting.value === 'true'}
        onchange={toggleChange}
        disabled={isSaving}
        label={setting.label}
      />
      <span class="muted">{setting.value === 'true' ? 'Enabled' : 'Disabled'}</span>
    </div>
  {:else if setting.type === 'readonly'}
    <div class="field-row">
      <span class="readonly-value">{setting.value || '—'}</span>
    </div>
  {:else if setting.type === 'button'}
    <div class="field-row">
      <button class="button" onclick={() => onUpdate(setting.key, '')} disabled={isSaving}>
        {setting.label}
      </button>
    </div>
  {:else if setting.type === 'select'}
    {#if editing}
      <div class="field-row">
        <select bind:value={editValue}>
          {#each (setting.options ?? []) as option}
            <option value={option}>{option}</option>
          {/each}
        </select>
        <div class="field-actions">
          <button class="button" onclick={save} disabled={isSaving}>Save</button>
          <button class="button danger" onclick={cancel}>Cancel</button>
        </div>
      </div>
    {:else}
      <div class="field-row">
        <span class="readonly-value">{setting.value || '—'}</span>
        <button class="button" onclick={startEdit}>Edit</button>
      </div>
    {/if}
  {:else if setting.type === 'multi-checkbox'}
    <div class="multi-checkbox-group">
      {#each (setting.options ?? []) as option}
        <label>
          <input
            type="checkbox"
            checked={checkboxValues.includes(option)}
            onchange={() => toggleCheckbox(option)}
          />
          {option}
        </label>
      {/each}
    </div>
    {#if checkboxChanged}
      <div class="field-row">
        <div class="field-actions">
          <button class="button" onclick={saveCheckboxes} disabled={isSaving}>Save</button>
          <button class="button danger" onclick={cancelCheckboxes}>Cancel</button>
        </div>
      </div>
    {/if}
  {:else if setting.type === 'password'}
    {#if editing}
      <div class="field-row">
        <input
          type={showPassword ? 'text' : 'password'}
          bind:value={editValue}
        />
        <button
          class="button"
          onclick={() => (showPassword = !showPassword)}
          type="button"
        >
          {showPassword ? 'Hide' : 'Show'}
        </button>
        <div class="field-actions">
          <button class="button" onclick={save} disabled={isSaving}>Save</button>
          <button class="button danger" onclick={cancel}>Cancel</button>
        </div>
      </div>
    {:else}
      <div class="field-row">
        <span class="readonly-value">••••••••</span>
        <button class="button" onclick={startEdit}>Edit</button>
      </div>
    {/if}
  {:else}
    <!-- text / number -->
    {#if editing}
      <div class="field-row">
        <input
          type={setting.type}
          bind:value={editValue}
        />
        <div class="field-actions">
          <button class="button" onclick={save} disabled={isSaving}>Save</button>
          <button class="button danger" onclick={cancel}>Cancel</button>
        </div>
      </div>
    {:else}
      <div class="field-row">
        <span class="readonly-value">{setting.value || '—'}</span>
        <button class="button" onclick={startEdit}>Edit</button>
      </div>
    {/if}
  {/if}

  {#if error}
    <span class="error">{error}</span>
  {/if}
</div>
