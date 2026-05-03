<script lang="ts">
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import { Link } from 'lucide-svelte';
</script>

<div class="space-y-6">
  <EmptyState
    icon={Link}
    title="Active sessions"
    description="Future session visibility and disconnect actions will use a backend ocserv integration interface with fakes in tests."
    items={[
      'No management socket access occurs in the frontend.',
      'Session termination will require explicit confirmation and audit logging.',
      'Missing socket or permission errors will be surfaced safely.'
    ]}
  />
</div>
