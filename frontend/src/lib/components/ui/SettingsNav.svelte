<script lang="ts">
  import type { SettingsSection } from '$lib/api';

  type Props = {
    sections: SettingsSection[];
    activeSection: string;
  };

  let { sections, activeSection }: Props = $props();
</script>

<nav class="flex gap-2 overflow-x-auto py-2 sticky top-0 z-10 bg-slate-50" aria-label="Settings sections">
  {#each sections as section}
    <a
      href="#{section.id}"
      class="whitespace-nowrap px-4 py-2 rounded-full text-sm font-semibold transition-colors {activeSection === section.id ? 'bg-slate-600 text-white' : 'bg-white border border-slate-200 text-slate-600 hover:bg-slate-100'}"
    >
      {section.label}
    </a>
  {/each}
</nav>
