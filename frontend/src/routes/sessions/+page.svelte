<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="Sessions"
  title="Active sessions"
  description="Future session visibility and disconnect actions will use a backend ocserv integration interface with fakes in tests."
  items={[
    'No management socket access occurs in the frontend.',
    'Session termination will require explicit confirmation and audit logging.',
    'Missing socket or permission errors will be surfaced safely.'
  ]}
/>
