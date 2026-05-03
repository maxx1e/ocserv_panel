<script lang="ts">
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import { FileText } from 'lucide-svelte';
</script>

<div class="space-y-6">
  <EmptyState
    icon={FileText}
    title="Audit log"
    description="Future administrative actions that mutate users, sessions, config, certificates, or service state will produce audit records."
    items={[
      'Denied actions should be auditable where practical.',
      'Audit records should avoid secrets and sensitive payloads.',
      'The frontend will read audit history through backend APIs.'
    ]}
  />
</div>
