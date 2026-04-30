<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="Audit"
  title="Audit log"
  description="Future administrative actions that mutate users, sessions, config, certificates, or service state will produce audit records."
  items={[
    'Denied actions should be auditable where practical.',
    'Audit records should avoid secrets and sensitive payloads.',
    'The frontend will read audit history through backend APIs.'
  ]}
/>
