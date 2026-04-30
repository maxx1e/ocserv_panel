<script lang="ts">
  type Props = {
    eyebrow?: string;
    title: string;
    description: string;
    items?: string[];
  };

  let { eyebrow = 'Placeholder', title, description, items = [] }: Props = $props();
</script>

<section class="panel">
  <p class="eyebrow">{eyebrow}</p>
  <h1>{title}</h1>
  <p class="muted">{description}</p>

  {#if items.length > 0}
    <ul>
      {#each items as item}
        <li>{item}</li>
      {/each}
    </ul>
  {/if}
</section>
