<script lang="ts">
  import { cn } from '$lib/utils';
  import type { Snippet } from 'svelte';
  import { Tooltip as TooltipPrimitive } from 'layerchart';

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  type TooltipData = Record<string, any>;

  type Props = {
    class?: string;
    label?: string;
    valueFormatter?: (value: number) => string;
    content?: Snippet<[{ data: TooltipData }]>;
  };

  let { class: className, label, valueFormatter, content }: Props = $props();
</script>

<TooltipPrimitive.Root variant="none">
  {#snippet children({ data }: { data: TooltipData })}
    <div
      class={cn(
        "border-border/50 bg-card text-card-foreground grid min-w-[9rem] items-start gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs shadow-xl",
        className
      )}
    >
      {#if content}
        {@render content({ data })}
      {:else if data}
        {#if data.timestamp}
          <div class="font-medium">
            {data.timestamp instanceof Date
              ? data.timestamp.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
              : String(data.timestamp)}
          </div>
        {/if}
        <div class="flex items-center justify-between gap-4">
          <span class="text-muted-foreground">{label ?? 'Value'}</span>
          <span class="font-mono font-medium tabular-nums">
            {valueFormatter && typeof data.value === 'number'
              ? valueFormatter(data.value)
              : data.value?.toLocaleString() ?? '--'}
          </span>
        </div>
      {/if}
    </div>
  {/snippet}
</TooltipPrimitive.Root>
