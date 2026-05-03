<script lang="ts">
  type Props = {
    checked: boolean;
    onchange: (value: boolean) => void;
    disabled?: boolean;
    label?: string;
  };

  let { checked, onchange, disabled = false, label = 'Toggle' }: Props = $props();
</script>

<button
  class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors {checked ? 'bg-slate-600' : 'bg-slate-300'} {disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}"
  {disabled}
  onclick={() => onchange(!checked)}
  role="switch"
  aria-checked={checked}
  aria-label={label}
  type="button"
>
  <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {checked ? 'translate-x-6' : 'translate-x-1'}"></span>
</button>
