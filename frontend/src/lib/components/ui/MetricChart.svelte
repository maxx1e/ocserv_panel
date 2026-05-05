<script lang="ts">
  import { Area, Axis, Chart, Highlight, LinearGradient, Svg } from 'layerchart';
  import { Container as ChartContainer, Tooltip as ChartTooltip } from '$lib/components/ui/chart/index';
  import {
    formatMetricValue,
    formatXAxisTime,
    getChartClasses,
    getChartConfig,
    getChartColorVar,
    getXTickCount,
  } from '$lib/monitoring';
  import type { ChartConfig } from '$lib/components/ui/chart/chart-utils';

  type TimeData = { timestamp: Date; value: number };

  type Props = {
    /** Sorted array of time-value pairs. Empty array -> placeholder shown. */
    data: TimeData[];
    /** Backend metric name used to derive colours, domain, and axis formatting. */
    metric: string;
    /** Pixel height of the chart container. Defaults to 160. */
    height?: number;
  };

  let { data, metric, height = 160 }: Props = $props();

  const config = $derived(getChartConfig(metric));
  const classes = $derived(getChartClasses(metric));
  const xTickCount = $derived(getXTickCount(data.length));
  const chartColor = $derived(getChartColorVar(metric));

  const chartConfig: ChartConfig = $derived({
    value: { label: metric, color: chartColor },
  });

  function tooltipFormatter(value: number): string {
    return formatMetricValue(metric, value);
  }
</script>

<div style="height: {height}px" class="w-full">
  {#if data.length > 0}
    <ChartContainer config={chartConfig} class="h-full w-full">
      <Chart
        {data}
        x="timestamp"
        y="value"
        yDomain={config.yDomain}
        yNice={config.yNice}
        padding={{ left: 44, bottom: 24, top: 8, right: 8 }}
        tooltip={{ mode: 'bisect-x' }}
      >
        <Svg>
          <Axis
            placement="left"
            grid
            format={config.yAxisFormat}
          />
          <Axis
            placement="bottom"
            format={(d: Date | number) => formatXAxisTime(d)}
            ticks={xTickCount}
          />
          <LinearGradient
            stops={[classes.gradientTop, classes.gradientBottom]}
            vertical
          >
            {#snippet children({ gradient }: { gradient: string })}
              <Area fill={gradient} line={{ class: classes.stroke }} />
            {/snippet}
          </LinearGradient>
          <Highlight points lines />
        </Svg>
        <ChartTooltip
          label={metric.replace(/_/g, ' ')}
          valueFormatter={tooltipFormatter}
        />
      </Chart>
    </ChartContainer>
  {:else}
    <div class="h-full flex items-center justify-center rounded-lg border border-dashed border-border bg-muted/30">
      <p class="text-sm text-muted-foreground">No data available</p>
    </div>
  {/if}
</div>
