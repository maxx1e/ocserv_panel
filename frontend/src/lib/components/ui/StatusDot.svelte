<script lang="ts">
  type Props = {
    status: string;
    size?: 'sm' | 'md';
  };

  let { status, size = 'sm' }: Props = $props();

  const GREEN = new Set(['running', 'active', 'success', 'ok']);
  const RED = new Set(['stopped', 'failure', 'error', 'failed']);
  const AMBER = new Set(['not_configured', 'unknown', 'warning', 'starting', 'stopping']);

  const colorClass = $derived(
    GREEN.has(status)
      ? 'bg-emerald-500'
      : RED.has(status)
        ? 'bg-red-500'
        : AMBER.has(status)
          ? 'bg-amber-400'
          : 'bg-slate-400'
  );

  const sizeClass = $derived(size === 'md' ? 'w-3 h-3' : 'w-2.5 h-2.5');
</script>

<span
  role="img"
  class="inline-block rounded-full flex-shrink-0 {colorClass} {sizeClass}"
  aria-label="{status} status"
  title={status}
></span>
