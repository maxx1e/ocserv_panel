<script lang="ts">
  import { cn } from '$lib/utils';
  import type { HTMLAttributes } from 'svelte/elements';
  import type { Snippet } from 'svelte';
  import ChartStyle from './chart-style.svelte';
  import { setChartContext, type ChartConfig } from './chart-utils';

  const uid = crypto.randomUUID().slice(0, 8);

  let {
    id,
    class: className,
    children,
    config,
    ...restProps
  }: HTMLAttributes<HTMLDivElement> & {
    config: ChartConfig;
    children?: Snippet;
    id?: string;
  } = $props();

  const chartId = $derived(`chart-${id || uid}`);

  setChartContext({
    get config() {
      return config;
    },
  });
</script>

<div
  data-chart={chartId}
  class={cn(
    "flex justify-center overflow-visible text-xs",
    "[&_.lc-highlight-point]:stroke-transparent",
    "[&_.lc-line]:stroke-border/50",
    "[&_.lc-highlight-line]:stroke-0",
    "[&_.lc-area-path]:opacity-100 [&_.lc-highlight-line]:opacity-100 [&_.lc-highlight-point]:opacity-100 [&_.lc-spline-path]:opacity-100",
    "[&_.lc-text]:text-xs [&_.lc-text-svg]:overflow-visible",
    "[&_.lc-axis-tick]:stroke-0",
    "[&_.lc-rule-x-line:not(.lc-grid-x-rule)]:stroke-0 [&_.lc-rule-y-line:not(.lc-grid-y-rule)]:stroke-0",
    "[&_.lc-grid-x-radial-line]:stroke-border [&_.lc-grid-x-radial-circle]:stroke-border",
    "[&_.lc-grid-y-radial-line]:stroke-border [&_.lc-grid-y-radial-circle]:stroke-border",
    "[&_.lc-axis-tick-label]:fill-muted-foreground [&_.lc-axis-tick-label]:font-normal",
    "[&_.lc-root-container]:w-full",
    "[&_.lc-tooltip-rects-g]:fill-transparent",
    "[&_.lc-layout-svg-g]:fill-transparent",
    "[&_.lc-labels-text:not([fill])]:fill-foreground [&_text]:stroke-transparent",
    className
  )}
  {...restProps}
>
  <ChartStyle id={chartId} {config} />
  {#if children}
    {@render children()}
  {/if}
</div>
