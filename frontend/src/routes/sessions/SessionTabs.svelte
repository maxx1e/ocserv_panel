<script lang="ts">
  type TabDef = {
    id: string;
    label: string;
    badge?: number;
  };

  type Props = {
    tabs: TabDef[];
    activeTab: string;
    onTabChange: (tabId: string) => void;
  };

  let { tabs, activeTab, onTabChange }: Props = $props();
</script>

<div class="border-b border-slate-200" role="tablist" aria-label="Sessions navigation">
  <nav class="flex gap-0 -mb-px overflow-x-auto">
    {#each tabs as tab (tab.id)}
      <button
        type="button"
        role="tab"
        id="tab-{tab.id}"
        aria-selected={activeTab === tab.id}
        aria-controls="tabpanel-{tab.id}"
        class="relative px-4 py-3 text-sm font-medium whitespace-nowrap transition-colors
          {activeTab === tab.id
            ? 'text-blue-600 border-b-2 border-blue-600'
            : 'text-slate-500 hover:text-slate-700 hover:border-b-2 hover:border-slate-300'}"
        onclick={() => onTabChange(tab.id)}
      >
        {tab.label}
        {#if tab.badge && tab.badge > 0}
          <span class="ml-1.5 inline-flex items-center justify-center px-1.5 py-0.5 text-xs font-medium rounded-full bg-red-100 text-red-700 min-w-[1.25rem]">
            {tab.badge}
          </span>
        {/if}
      </button>
    {/each}
  </nav>
</div>
