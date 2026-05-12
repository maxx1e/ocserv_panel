<script lang="ts">
  import { Chart, Sankey, Svg } from 'layerchart';
  // @ts-ignore -- d3-sankey ships without bundled types; layerchart depends on it at runtime
  import { sankeyLinkHorizontal } from 'd3-sankey';
  import { formatBytes } from '$lib/monitoring';

  type InterfaceEntry = { name: string; bytes: number };

  type Props = {
    interfaces: InterfaceEntry[];
    sourceLabel?: string;
    mode: 'rate' | 'cumulative';
    height?: number;
  };

  let { interfaces, sourceLabel = 'Total VPN', mode, height = 180 }: Props = $props();

  const IFACE_COLORS = [
    { solid: 'rgba(139,92,246,0.85)', fill: 'rgba(139,92,246,0.40)' },
    { solid: 'rgba(6,182,212,0.85)',  fill: 'rgba(6,182,212,0.40)' },
    { solid: 'rgba(245,158,11,0.85)', fill: 'rgba(245,158,11,0.40)' },
    { solid: 'rgba(244,63,94,0.85)',  fill: 'rgba(244,63,94,0.40)' },
    { solid: 'rgba(16,185,129,0.85)', fill: 'rgba(16,185,129,0.40)' },
    { solid: 'rgba(217,70,239,0.85)', fill: 'rgba(217,70,239,0.40)' },
  ];
  const SOURCE_COLOR = { solid: 'rgba(148,163,184,0.85)', fill: 'rgba(148,163,184,0.40)' };

  const total = $derived(interfaces.reduce((s, i) => s + i.bytes, 0));

  const sankeyData = $derived({
    nodes: [
      { id: '__source__', label: sourceLabel },
      ...interfaces.map(i => ({ id: i.name, label: i.name }))
    ],
    links: interfaces.map(i => ({
      source: '__source__',
      target: i.name,
      value: Math.max(i.bytes, 1),
    }))
  });

  function colorForIndex(index: number) {
    return IFACE_COLORS[index % IFACE_COLORS.length];
  }

  function formatValue(bytes: number): string {
    if (mode === 'rate') return `${formatBytes(bytes, 'iec')}/s`;
    return formatBytes(bytes, 'iec');
  }

  function pct(bytes: number): string {
    if (total === 0) return '0%';
    return `${Math.round((bytes / total) * 100)}%`;
  }

  const linkPath = sankeyLinkHorizontal();
</script>

<div style="height: {height}px" class="w-full">
  {#if interfaces.length === 0 || total === 0}
    <div class="h-full flex items-center justify-center rounded-lg border border-dashed border-slate-200 bg-slate-50/50">
      <p class="text-sm text-slate-400">No VPN interfaces active</p>
    </div>
  {:else}
    <div style="height: {height - 32}px" class="w-full">
      <Chart
        data={sankeyData}
        padding={{ left: 8, right: 120, top: 8, bottom: 8 }}
      >
        <Svg>
          <Sankey
            nodeId={(d: any) => d.id}
            nodeWidth={16}
            nodePadding={14}
            nodeAlign="left"
            let:nodes
            let:links
          >
            {#each links as link}
              {@const ifaceIdx = interfaces.findIndex(ifc => ifc.name === link.target.id)}
              {@const color = ifaceIdx >= 0 ? colorForIndex(ifaceIdx) : SOURCE_COLOR}
              <path
                d={linkPath(link) ?? ''}
                stroke={color.fill}
                stroke-width={Math.max(link.width ?? 1, 1)}
                fill="none"
              />
            {/each}
            {#each nodes as node}
              {@const isSource = node.id === '__source__'}
              {@const ifaceIdx = interfaces.findIndex(i => i.name === node.id)}
              {@const color = isSource ? SOURCE_COLOR : colorForIndex(ifaceIdx)}
              {@const nodeH = (node.y1 ?? 0) - (node.y0 ?? 0)}
              <rect
                x={node.x0 ?? 0}
                y={node.y0 ?? 0}
                width={(node.x1 ?? 0) - (node.x0 ?? 0)}
                height={Math.max(nodeH, 4)}
                rx="3"
                fill={color.solid}
              />
              {#if !isSource}
                {@const ifaceEntry = interfaces[ifaceIdx]}
                <text
                  x={(node.x1 ?? 0) + 8}
                  y={(node.y0 ?? 0) + nodeH / 2 - 5}
                  font-size="9"
                  font-weight="600"
                  fill="#374151"
                  dominant-baseline="middle"
                >{node.id}</text>
                <text
                  x={(node.x1 ?? 0) + 8}
                  y={(node.y0 ?? 0) + nodeH / 2 + 7}
                  font-size="8"
                  fill={color.solid}
                  dominant-baseline="middle"
                >{pct(ifaceEntry?.bytes ?? 0)}</text>
              {/if}
            {/each}
          </Sankey>
        </Svg>
      </Chart>
    </div>

    <div class="flex flex-wrap gap-x-4 gap-y-1 px-2 mt-1">
      {#each interfaces as iface, i}
        <div class="flex items-center gap-1.5 text-[10px] text-slate-500">
          <span
            class="inline-block w-2 h-2 rounded-full flex-shrink-0"
            style="background: {colorForIndex(i).solid}"
          ></span>
          <span>{iface.name}</span>
          <span class="font-medium text-slate-700">{formatValue(iface.bytes)}</span>
        </div>
      {/each}
    </div>
  {/if}
</div>
