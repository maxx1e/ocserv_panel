<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="Users"
  title="User lifecycle management"
  description="Future workflows will manage password and certificate-based ocserv users through authorized backend APIs."
  items={[
    'User creation and deletion will require RBAC authorization.',
    'Password handling will avoid plaintext storage.',
    'Certificate workflows will avoid storing private keys in SQLite.'
  ]}
/>
