<script lang="ts">
  let { title, reason }: { title: string; reason?: string | null } = $props();

  const displayReason = $derived(reason?.trim() ? reason : 'Not collecting yet');
</script>

<div class="unavailable-metric" role="status">
  <strong>{title}</strong>
  <p>{displayReason}</p>
</div>
