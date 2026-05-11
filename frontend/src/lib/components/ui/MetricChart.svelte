<script lang="ts">
  import { Area, Axis, Bars, Chart, Highlight, LinearGradient, Svg } from 'layerchart';
  // @ts-ignore -- d3-scale ships without bundled types; layerchart depends on it at runtime
  import { scaleBand } from 'd3-scale';
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
    /** Chart rendering style. 'line' renders an area chart, 'bar' renders vertical bars. Defaults to 'line'. */
    chartType?: 'line' | 'bar';
  };

  let { data, metric, height = 160, chartType = 'line' }: Props = $props();

  const config = $derived(getChartConfig(metric));
  const classes = $derived(getChartClasses(metric));
  const xTickCount = $derived(getXTickCount(data.length));
  const chartColor = $derived(getChartColorVar(metric));

  const chartConfig: ChartConfig = $derived({
    value: { label: metric, color: chartColor },
  });

  /** For bar charts, use scaleBand on the x-axis so Bars renders vertical columns. */
  const barXScale = $derived(chartType === 'bar' ? scaleBand() : undefined);

  /** Bar fill uses the gradient-top RGBA color (30% opacity looks good for solid bars). */
  const barFill = $derived(classes.gradientTop);

  /** Extra props for the Chart component when rendering bars (xScale override). */
  const chartScaleProps = $derived(
    chartType === 'bar' ? { xScale: barXScale } : {}
  );

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
        {...chartScaleProps}
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
          {#if chartType === 'bar'}
            <Bars fill={barFill} radius={2} strokeWidth={0} />
          {:else}
            <LinearGradient
              stops={[classes.gradientTop, classes.gradientBottom]}
              vertical
            >
              {#snippet children({ gradient }: { gradient: string })}
                <Area fill={gradient} line={{ class: classes.stroke }} />
              {/snippet}
            </LinearGradient>
          {/if}
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
