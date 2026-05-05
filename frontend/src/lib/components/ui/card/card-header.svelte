<script lang="ts">
  import { cn } from '$lib/utils';
  import type { HTMLAttributes } from 'svelte/elements';
  import type { Snippet } from 'svelte';

  let {
    class: className,
    children,
    ...restProps
  }: HTMLAttributes<HTMLDivElement> & { children?: Snippet } = $props();
</script>

<div
  class={cn(
    "flex items-center justify-between gap-2 px-6",
    className
  )}
  {...restProps}
>
  {#if children}
    {@render children()}
  {/if}
</div>
