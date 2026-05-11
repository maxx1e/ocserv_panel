<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { Snippet } from 'svelte';

  export type TooltipContent = {
    title?: string;
    description: string;
    action?: string;
  };

  type Props = {
    content: TooltipContent | null | undefined;
    children: Snippet;
  };

  let { content, children }: Props = $props();

  let showTooltip = $state(false);
  let hoverTimeout: ReturnType<typeof setTimeout> | null = null;
  const tooltipId = `health-tooltip-${Math.random().toString(36).slice(2, 9)}`;

  function show() {
    if (!content) return;
    hoverTimeout = setTimeout(() => { showTooltip = true; }, 150);
  }

  function hide() {
    if (hoverTimeout) { clearTimeout(hoverTimeout); hoverTimeout = null; }
    showTooltip = false;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') hide();
  }

  onDestroy(() => {
    if (hoverTimeout) { clearTimeout(hoverTimeout); hoverTimeout = null; }
  });
</script>

<span
  class="relative inline-flex"
  role="group"
  onmouseenter={show}
  onmouseleave={hide}
  onfocusin={show}
  onfocusout={hide}
  onkeydown={handleKeydown}
  aria-describedby={showTooltip && content ? tooltipId : undefined}
>
  {@render children()}
  {#if showTooltip && content}
    <div
      id={tooltipId}
      role="tooltip"
      class="absolute z-50 bottom-full left-1/2 -translate-x-1/2 mb-2 w-64 px-3 py-2.5 text-xs bg-slate-800 text-white rounded-lg shadow-lg"
    >
      {#if content.title}
        <p class="font-semibold mb-1">{content.title}</p>
      {/if}
      <p class="text-slate-200">{content.description}</p>
      {#if content.action}
        <p class="mt-1.5 text-amber-300 font-medium">{content.action}</p>
      {/if}
      <div class="absolute top-full left-1/2 -translate-x-1/2 -mt-1 w-2 h-2 bg-slate-800 rotate-45"></div>
    </div>
  {/if}
</span>
