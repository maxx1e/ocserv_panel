<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="Settings"
  title="Panel settings"
  description="Future local settings will be stored in SQLite and managed through backend APIs."
  items={[
    'Settings must not contain plaintext secrets.',
    'Operational paths and permission assumptions must be documented.',
    'Runtime changes that affect services require safe validation and audit paths.'
  ]}
/>
