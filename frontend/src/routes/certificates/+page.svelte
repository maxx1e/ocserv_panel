<script lang="ts">
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import { ShieldCheck } from 'lucide-svelte';
</script>

<div class="space-y-6">
  <EmptyState
    icon={ShieldCheck}
    title="Certificate lifecycle"
    description="Future certificate views will show safe metadata, expiry, issuance, and revocation status without storing private keys."
    items={[
      'Private keys must not be stored in source control or SQLite.',
      'Revocation and issuance will require backend authorization.',
      'Certificate filesystem access must be isolated behind interfaces.'
    ]}
  />
</div>
