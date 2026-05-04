<script lang="ts">
  import { Area, Axis, Chart, Highlight, LinearGradient, Svg, Tooltip } from 'layerchart';
  import { formatMetricValue, formatXAxisTime, getChartClasses, getChartConfig, getXTickCount } from '$lib/monitoring';

  type TimeData = { timestamp: Date; value: number };

  type Props = {
    /** Sorted array of time-value pairs. Empty array → placeholder shown. */
    data: TimeData[];
    /** Backend metric name used to derive colours, domain, and axis formatting. */
    metric: string;
    /** Pixel height of the chart container. Defaults to 200. */
    height?: number;
  };

  let { data, metric, height = 200 }: Props = $props();

  const config = $derived(getChartConfig(metric));
  const classes = $derived(getChartClasses(metric));
  const xTickCount = $derived(getXTickCount(data.length));

  function tooltipTime(d: Date): string {
    return d.toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  }
</script>

<div style="height: {height}px">
  {#if data.length > 0}
    <Chart
      {data}
      x="timestamp"
      y="value"
      yDomain={config.yDomain}
      yNice={config.yNice}
      padding={{ left: 52, bottom: 32, top: 12, right: 12 }}
      tooltip={{ mode: 'bisect-x' }}
    >
      <Svg>
        <!-- Y-axis: left side with grid lines -->
        <Axis
          placement="left"
          grid
          rule
          format={config.yAxisFormat}
        />
        <!-- X-axis: bottom side with time formatting -->
        <Axis
          placement="bottom"
          grid
          rule
          format={(d: Date | number) => formatXAxisTime(d)}
          ticks={xTickCount}
        />
        <!-- Coloured gradient under the line -->
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
      <Tooltip.Root>
        {#snippet children({ data }: { data: TimeData })}
          <Tooltip.Header>{tooltipTime(data.timestamp)}</Tooltip.Header>
          <Tooltip.List>
            <Tooltip.Item label="Value" value={formatMetricValue(metric, data.value)} />
          </Tooltip.List>
        {/snippet}
      </Tooltip.Root>
    </Chart>
  {:else}
    <div class="h-full flex items-center justify-center rounded-md border border-dashed border-slate-200 bg-slate-50">
      <p class="text-sm text-slate-400">No data available</p>
    </div>
  {/if}
</div>
