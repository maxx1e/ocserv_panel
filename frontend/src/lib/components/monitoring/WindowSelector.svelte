<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  let { value, windows }: { value: string; windows: string[] } = $props();

  const dispatch = createEventDispatcher<{ change: string }>();

  function selectWindow(window: string) {
    if (window === value) return;
    dispatch('change', window);
  }
</script>

<div class="window-selector" aria-label="Monitoring window">
  {#each windows as window}
    <button
      type="button"
      class:active={window === value}
      aria-pressed={window === value}
      onclick={() => selectWindow(window)}
    >
      {window}
    </button>
  {/each}
</div>
