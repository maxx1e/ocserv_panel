<script lang="ts">
  import type { SettingDef } from '$lib/api';
  import SettingToggle from './SettingToggle.svelte';
  import SettingSelect from './SettingSelect.svelte';
  import SettingText from './SettingText.svelte';
  import SettingNumber from './SettingNumber.svelte';
  import SettingPassword from './SettingPassword.svelte';
  import SettingMultiCheckbox from './SettingMultiCheckbox.svelte';
  import SettingReadonly from './SettingReadonly.svelte';
  import SettingButton from './SettingButton.svelte';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<SettingDef>;
    savingKey: string | null;
    configHealthy?: boolean;
  };

  let { setting, onUpdate, savingKey, configHealthy = true }: Props = $props();

  // Config-source settings are disabled when config file has issues
  const disabled = $derived(!configHealthy && setting.source === 'config');
</script>

{#if setting.type === 'toggle'}
  <SettingToggle {setting} {onUpdate} {savingKey} {disabled} />
{:else if setting.type === 'select'}
  <SettingSelect {setting} {onUpdate} {savingKey} {disabled} />
{:else if setting.type === 'text'}
  <SettingText {setting} {onUpdate} {savingKey} {disabled} />
{:else if setting.type === 'number'}
  <SettingNumber {setting} {onUpdate} {savingKey} {disabled} />
{:else if setting.type === 'password'}
  <SettingPassword {setting} {onUpdate} {savingKey} {disabled} />
{:else if setting.type === 'multi-checkbox'}
  <SettingMultiCheckbox {setting} {onUpdate} {savingKey} {disabled} />
{:else if setting.type === 'readonly'}
  <SettingReadonly {setting} />
{:else if setting.type === 'button'}
  <SettingButton {setting} {onUpdate} {savingKey} {disabled} />
{/if}
