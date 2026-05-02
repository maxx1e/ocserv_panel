<script lang="ts">
  type Props = {
    label: string;
    value: string;
    unit?: string;
  };

  let { label, value, unit }: Props = $props();
</script>

<article class="analytics-card panel">
  <span class="analytics-label">{label}</span>
  <strong class="analytics-value">{value}</strong>
  {#if unit}
    <small class="analytics-unit">{unit}</small>
  {/if}
</article>

<style>
  .analytics-card {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .analytics-label {
    font-size: 0.875rem;
    color: var(--color-muted, #888);
  }
  .analytics-value {
    font-size: 1.5rem;
    line-height: 1.2;
  }
  .analytics-unit {
    font-size: 0.75rem;
    color: var(--color-muted, #888);
  }
</style>
