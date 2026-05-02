<script lang="ts">
  interface Props {
    checked: boolean;
    onchange: (value: boolean) => void;
    disabled?: boolean;
    label?: string;
  }

  let { checked, onchange, disabled = false, label = 'Toggle' }: Props = $props();
</script>

<button
  class="toggle-switch"
  class:active={checked}
  {disabled}
  onclick={() => onchange(!checked)}
  role="switch"
  aria-checked={checked}
  aria-label={label}
  type="button"
>
  <span class="toggle-thumb"></span>
</button>
