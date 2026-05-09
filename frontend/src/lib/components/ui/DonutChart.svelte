<script lang="ts">
  type Segment = { label: string; value: number; color: string };
  type Props = {
    segments: Segment[];
    title?: string;
    height?: number;
  };

  let { segments, title, height = 200 }: Props = $props();

  const total = $derived(segments.reduce((sum, s) => sum + s.value, 0));

  type ArcSegment = Segment & { path: string };

  const arcs: ArcSegment[] = $derived.by(() => {
    if (total === 0) return [];
    const cx = 100;
    const cy = 100;
    const outerR = 90;
    const innerR = 55;
    let startAngle = -Math.PI / 2;
    return segments.map((seg) => {
      const angle = (seg.value / total) * 2 * Math.PI;
      const endAngle = startAngle + angle;
      const x1 = cx + outerR * Math.cos(startAngle);
      const y1 = cy + outerR * Math.sin(startAngle);
      const x2 = cx + outerR * Math.cos(endAngle);
      const y2 = cy + outerR * Math.sin(endAngle);
      const ix1 = cx + innerR * Math.cos(endAngle);
      const iy1 = cy + innerR * Math.sin(endAngle);
      const ix2 = cx + innerR * Math.cos(startAngle);
      const iy2 = cy + innerR * Math.sin(startAngle);
      const largeArc = angle > Math.PI ? 1 : 0;
      const path = [
        `M ${x1} ${y1}`,
        `A ${outerR} ${outerR} 0 ${largeArc} 1 ${x2} ${y2}`,
        `L ${ix1} ${iy1}`,
        `A ${innerR} ${innerR} 0 ${largeArc} 0 ${ix2} ${iy2}`,
        'Z',
      ].join(' ');
      const result: ArcSegment = { ...seg, path };
      startAngle = endAngle;
      return result;
    });
  });
</script>

<div style="height: {height}px" class="w-full flex flex-col items-center">
  {#if title}
    <p class="text-sm font-medium text-foreground mb-2">{title}</p>
  {/if}

  {#if segments.length === 0 || total === 0}
    <div class="flex-1 w-full flex items-center justify-center rounded-lg border border-dashed border-border bg-muted/30">
      <p class="text-sm text-muted-foreground">No session data</p>
    </div>
  {:else}
    <div class="flex-1 flex items-center justify-center">
      <svg viewBox="0 0 200 200" class="w-full h-full max-w-[160px] max-h-[160px]">
        {#each arcs as arc}
          <path d={arc.path} fill={arc.color} class="transition-opacity hover:opacity-80" />
        {/each}
        <text
          x="100"
          y="96"
          text-anchor="middle"
          class="fill-foreground text-[1.5rem] font-semibold"
        >
          {total}
        </text>
        <text
          x="100"
          y="116"
          text-anchor="middle"
          class="fill-muted-foreground text-[0.6rem]"
        >
          sessions
        </text>
      </svg>
    </div>

    <div class="flex flex-wrap justify-center gap-x-4 gap-y-1 mt-2">
      {#each segments as seg}
        <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
          <span class="inline-block w-2.5 h-2.5 rounded-full" style="background-color: {seg.color}"></span>
          <span>{seg.label}</span>
          <span class="font-medium text-foreground">{seg.value}</span>
        </div>
      {/each}
    </div>
  {/if}
</div>
