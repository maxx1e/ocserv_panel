<script lang="ts">
  import type { ChartConfig } from './chart-utils';

  type Props = {
    id: string;
    config: ChartConfig;
  };

  let { id, config }: Props = $props();

  function generateColorCSS(): string {
    const lines: string[] = [];

    // Light mode
    lines.push(`[data-chart="${id}"] {`);
    for (const [key, value] of Object.entries(config)) {
      if (value.color) {
        lines.push(`  --color-${key}: ${value.color};`);
      } else if (value.theme?.light) {
        lines.push(`  --color-${key}: ${value.theme.light};`);
      }
    }
    lines.push('}');

    // Dark mode
    const darkEntries = Object.entries(config).filter(([, v]) => v.theme?.dark);
    if (darkEntries.length > 0) {
      lines.push(`.dark [data-chart="${id}"] {`);
      for (const [key, value] of darkEntries) {
        lines.push(`  --color-${key}: ${value.theme!.dark};`);
      }
      lines.push('}');
    }

    return lines.join('\n');
  }
</script>

{@html `<style>${generateColorCSS()}</style>`}
