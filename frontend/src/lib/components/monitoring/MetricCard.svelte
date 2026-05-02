<script lang="ts">
  type MetricState = 'collecting' | 'unavailable';

  let { label, value, hint, state = 'collecting' }: {
    label: string;
    value: string;
    hint?: string;
    state?: MetricState;
  } = $props();

  const displayValue = $derived(value?.trim() ? value : '--');
</script>

<article class:unavailable={state === 'unavailable'} class="metric-card panel" aria-live="polite">
  <span>{label}</span>
  <strong>{displayValue}</strong>
  {#if hint}
    <small>{hint}</small>
  {/if}
</article>
