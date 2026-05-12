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
    /** Optional custom formatter for X-axis labels on bar charts. Defaults to HH:MM. */
    xLabelFormat?: (d: string) => string;
  };

  let { data, metric, height = 160, chartType = 'line', xLabelFormat }: Props = $props();

  const config = $derived(getChartConfig(metric));
  const classes = $derived(getChartClasses(metric));
  const xTickCount = $derived(getXTickCount(data.length));
  const chartColor = $derived(getChartColorVar(metric));

  const chartConfig: ChartConfig = $derived({
    value: { label: metric, color: chartColor },
  });

  /**
   * For bar charts, scaleBand requires discrete ordinal domain values (strings).
   * Convert timestamps to ISO strings so each data point maps to a distinct band.
   */
  const barData = $derived(
    chartType === 'bar'
      ? data.map((d) => ({ ...d, xKey: d.timestamp.toISOString() }))
      : []
  );

  /** Ordinal domain for the band scale — one string per data point. */
  const barXDomain = $derived(barData.map((d) => d.xKey));

  /** scaleBand instance with the correct discrete domain. Range is set by LayerCake. */
  const barXScale = $derived(
    chartType === 'bar' ? scaleBand().domain(barXDomain).padding(0.1) : undefined
  );

  /** Bar fill uses the gradient-top RGBA color. */
  const barFill = $derived(classes.gradientTop);

  function tooltipFormatter(value: number): string {
    return formatMetricValue(metric, value);
  }
</script>

<div style="height: {height}px" class="w-full">
  {#if data.length > 0}
    <ChartContainer config={chartConfig} class="h-full w-full">
      {#if chartType === 'bar'}
        <Chart
          data={barData}
          x="xKey"
          xScale={barXScale}
          y="value"
          yDomain={config.yDomain}
          yNice={config.yNice}
          padding={{ left: 44, bottom: 24, top: 8, right: 8 }}
          tooltip={{ mode: 'band' }}
        >
          <Svg>
            <Axis
              placement="left"
              grid
              format={config.yAxisFormat}
            />
            <Axis
              placement="bottom"
              format={(d: string) => xLabelFormat ? xLabelFormat(d) : formatXAxisTime(new Date(d))}
              ticks={xTickCount}
            />
            <Bars fill={barFill} radius={2} strokeWidth={0} />
            <Highlight points lines />
          </Svg>
          <ChartTooltip
            label={metric.replace(/_/g, ' ')}
            valueFormatter={tooltipFormatter}
          />
        </Chart>
      {:else}
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
      {/if}
    </ChartContainer>
  {:else}
    <div class="h-full flex items-center justify-center rounded-lg border border-dashed border-border bg-muted/30">
      <p class="text-sm text-muted-foreground">No data available</p>
    </div>
  {/if}
</div>
