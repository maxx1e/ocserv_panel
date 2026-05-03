<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
  };

  let { setting }: Props = $props();
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart}>
  <span class="text-sm text-slate-600">{setting.value || '—'}</span>
</SettingRow>
