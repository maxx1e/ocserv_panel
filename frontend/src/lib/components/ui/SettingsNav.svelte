<script lang="ts">
  import type { SettingsSection } from '$lib/api';

  type Props = {
    sections: SettingsSection[];
    activeSection: string;
    mobile?: boolean;
  };

  let { sections, activeSection, mobile = false }: Props = $props();
</script>

{#if mobile}
  <nav class="flex gap-2 overflow-x-auto py-2 bg-slate-50" aria-label="Settings sections">
    {#each sections as section}
      <a
        href="#{section.id}"
        class="whitespace-nowrap px-4 py-2 rounded-full text-sm font-semibold transition-colors {activeSection === section.id ? 'bg-slate-600 text-white' : 'bg-white border border-slate-200 text-slate-600 hover:bg-slate-100'}"
      >
        {section.label}
      </a>
    {/each}
  </nav>
{:else}
  <nav class="flex flex-col gap-0.5" aria-label="Settings sections">
    {#each sections as section}
      <a
        href="#{section.id}"
        class="flex items-center px-3 py-2 rounded-lg text-sm transition-colors
          {activeSection === section.id
            ? 'bg-slate-100 text-slate-900 font-medium border-l-2 border-slate-600 pl-[10px]'
            : 'text-slate-600 hover:bg-slate-50 hover:text-slate-800'}"
      >
        {section.label}
      </a>
    {/each}
  </nav>
{/if}
