<script lang="ts">
  interface Props {
    pendingChanges: string[];
    onRestart: () => void;
    restarting: boolean;
  }

  let { pendingChanges, onRestart, restarting }: Props = $props();
</script>

{#if pendingChanges.length > 0}
  <div class="callout warning">
    <strong>Restart required</strong>
    <p>
      {restarting
        ? 'Restarting… the page will reload when the service is back.'
        : 'Configuration changes need a service restart to take effect.'}
    </p>
    {#if !restarting}
      <ul>
        {#each pendingChanges as change}
          <li>{change}</li>
        {/each}
      </ul>
      <button class="button danger" onclick={onRestart}>Restart Now</button>
    {/if}
  </div>
{/if}
