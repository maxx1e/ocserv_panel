<script lang="ts">
  import type { SettingsSection as SettingsSectionType } from '$lib/api';
  import SettingField from './SettingField.svelte';

  interface Props {
    section: SettingsSectionType;
    onUpdate: (key: string, value: string) => Promise<void>;
    savingKey: string | null;
  }

  let { section, onUpdate, savingKey }: Props = $props();
</script>

<section id={section.id} class="panel settings-section">
  <p class="eyebrow">{section.label}</p>
  <h3>{section.title}</h3>
  {#if section.description}
    <p class="muted">{section.description}</p>
  {/if}
  <div class="form-stack">
    {#each section.settings as setting (setting.key)}
      <SettingField {setting} {onUpdate} {savingKey} />
    {/each}
  </div>
</section>
