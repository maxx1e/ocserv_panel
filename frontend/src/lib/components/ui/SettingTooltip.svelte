<script lang="ts">
  type Props = {
    description?: string;
  };

  let { description }: Props = $props();

  let showTooltip = $state(false);
  let hoverTimeout: ReturnType<typeof setTimeout> | null = null;

  function handleMouseEnter() {
    hoverTimeout = setTimeout(() => {
      showTooltip = true;
    }, 150);
  }

  function handleMouseLeave() {
    if (hoverTimeout) {
      clearTimeout(hoverTimeout);
      hoverTimeout = null;
    }
    showTooltip = false;
  }
</script>

{#if description}
  <span class="relative inline-flex items-center ml-1" role="button" tabindex="-1" onmouseenter={handleMouseEnter} onmouseleave={handleMouseLeave}>
    <svg
      class="w-3.5 h-3.5 text-slate-400 hover:text-slate-600 transition-colors cursor-help"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="2"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="10" />
      <path d="M12 16v-4" />
      <path d="M12 8h.01" />
    </svg>
    {#if showTooltip}
      <div
        class="absolute z-50 bottom-full left-1/2 -translate-x-1/2 mb-2 w-64 px-3 py-2 text-xs text-white bg-slate-800 rounded-lg shadow-lg"
        role="tooltip"
      >
        {description}
        <div class="absolute top-full left-1/2 -translate-x-1/2 -mt-1 w-2 h-2 bg-slate-800 rotate-45"></div>
      </div>
    {/if}
  </span>
{/if}
