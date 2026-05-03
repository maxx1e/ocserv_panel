<script lang="ts">
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import { Settings } from 'lucide-svelte';
</script>

<div class="space-y-6">
  <EmptyState
    icon={Settings}
    title="Safe configuration editing"
    description="Future config changes will require validation, backup, preview, and safe apply paths before touching live ocserv configuration."
    items={[
      'The baseline does not mutate live configuration.',
      'Future changes must validate before apply.',
      'Backups and rollback assumptions must be documented.'
    ]}
  />
</div>
