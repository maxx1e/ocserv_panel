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

<h3 class={cn("text-sm font-semibold leading-none", className)} {...restProps}>
  {#if children}
    {@render children()}
  {/if}
</h3>
