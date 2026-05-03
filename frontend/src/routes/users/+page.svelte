<script lang="ts">
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import { Users } from 'lucide-svelte';
</script>

<div class="space-y-6">
  <EmptyState
    icon={Users}
    title="User lifecycle management"
    description="Future workflows will manage password and certificate-based ocserv users through authorized backend APIs."
    items={[
      'User creation and deletion will require RBAC authorization.',
      'Password handling will avoid plaintext storage.',
      'Certificate workflows will avoid storing private keys in SQLite.'
    ]}
  />
</div>
