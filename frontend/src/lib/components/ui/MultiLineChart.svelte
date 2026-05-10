<script lang="ts">
  import { Area, Axis, Chart, Highlight, LinearGradient, Svg } from 'layerchart';
  import { Container as ChartContainer, Tooltip as ChartTooltip } from '$lib/components/ui/chart/index';
  import { formatMetricValue, formatXAxisTime, getChartConfig, getXTickCount } from '$lib/monitoring';

  export type LineData = {
    name: string;
    data: { timestamp: Date; value: number }[];
  };

  type Props = {
    lines: LineData[];
    metric: string;
    height?: number;
  };

  let { lines, metric, height = 160 }: Props = $props();

  const VPN_LINE_PALETTE = [
    { stroke: 'stroke-violet-500 stroke-2', top: 'rgba(139, 92, 246, 0.25)', bottom: 'rgba(139, 92, 246, 0.05)' },
    { stroke: 'stroke-fuchsia-500 stroke-2', top: 'rgba(217, 70, 239, 0.25)', bottom: 'rgba(217, 70, 239, 0.05)' },
    { stroke: 'stroke-rose-500 stroke-2', top: 'rgba(244, 63, 94, 0.25)', bottom: 'rgba(244, 63, 94, 0.05)' },
    { stroke: 'stroke-amber-500 stroke-2', top: 'rgba(245, 158, 11, 0.25)', bottom: 'rgba(245, 158, 11, 0.05)' },
  ];

  const config = $derived(getChartConfig(metric));

  const hasData = $derived(lines.some(l => l.data.length > 0));

  function colorFor(index: number) {
    return VPN_LINE_PALETTE[index % VPN_LINE_PALETTE.length];
  }

  function tooltipFormatter(value: number): string {
    return formatMetricValue(metric, value);
  }

  const allData = $derived(lines.flatMap(l => l.data));
  const xTickCount = $derived(getXTickCount(allData.length));
</script>

<div style="height: {height}px" class="w-full">
  {#if hasData}
    <div style="height: {lines.length > 1 ? height - 24 : height}px" class="w-full">
      <ChartContainer config={{}} class="h-full w-full">
        <Chart
          data={allData}
          x="timestamp"
          y="value"
          yDomain={config.yDomain}
          yNice={config.yNice}
          padding={{ left: 44, bottom: 24, top: 8, right: 8 }}
          tooltip={{ mode: 'bisect-x' }}
        >
          <Svg>
            <Axis placement="left" grid format={config.yAxisFormat} />
            <Axis
              placement="bottom"
              format={(d: Date | number) => formatXAxisTime(d)}
              ticks={xTickCount}
            />
            {#each lines as line, i}
              {#if line.data.length > 0}
                {@const colors = colorFor(i)}
                <LinearGradient
                  stops={[colors.top, colors.bottom]}
                  vertical
                >
                  {#snippet children({ gradient }: { gradient: string })}
                    <Area
                      data={line.data}
                      fill={gradient}
                      line={{ class: colors.stroke }}
                    />
                  {/snippet}
                </LinearGradient>
              {/if}
            {/each}
            <Highlight points lines />
          </Svg>
          <ChartTooltip
            label={metric.replace(/_/g, ' ')}
            valueFormatter={tooltipFormatter}
          />
        </Chart>
      </ChartContainer>
    </div>
    {#if lines.length > 1}
      <div class="flex flex-wrap gap-3 mt-1 px-1">
        {#each lines as line, i}
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <span class="inline-block w-2.5 h-2.5 rounded-full flex-shrink-0" style="background: {VPN_LINE_PALETTE[i % VPN_LINE_PALETTE.length].top.replace('0.25', '0.8')}"></span>
            {line.name}
          </div>
        {/each}
      </div>
    {/if}
  {:else}
    <div class="h-full flex items-center justify-center rounded-lg border border-dashed border-border bg-muted/30">
      <p class="text-sm text-muted-foreground">No data available</p>
    </div>
  {/if}
</div>
