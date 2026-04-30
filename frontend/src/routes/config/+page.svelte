<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="Config"
  title="Safe configuration editing"
  description="Future config changes will require validation, backup, preview, and safe apply paths before touching live ocserv configuration."
  items={[
    'The baseline does not mutate live configuration.',
    'Future changes must validate before apply.',
    'Backups and rollback assumptions must be documented.'
  ]}
/>
