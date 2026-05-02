<script lang="ts">
  import type { SettingsSection } from '$lib/api';

  interface Props {
    sections: SettingsSection[];
    activeSection: string;
  }

  let { sections, activeSection }: Props = $props();
</script>

<nav class="settings-nav" aria-label="Settings sections">
  {#each sections as section}
    <a href="#{section.id}" class:active={activeSection === section.id}>
      {section.label}
    </a>
  {/each}
</nav>
