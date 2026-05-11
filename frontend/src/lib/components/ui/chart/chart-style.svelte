<script lang="ts">
  import type { ChartConfig } from './chart-utils';

  type Props = {
    id: string;
    config: ChartConfig;
  };

  let { id, config }: Props = $props();

  /** Strip characters that could enable CSS injection (expressions, urls, semicolons). */
  function sanitizeCSSValue(value: string): string {
    return value.replace(/[^a-zA-Z0-9(),.\s%#\-]/g, '');
  }

  /** Strip characters that could break CSS selectors or property names. */
  function sanitizeCSSIdent(value: string): string {
    return value.replace(/[^a-zA-Z0-9\-_]/g, '');
  }

  function generateColorCSS(): string {
    const lines: string[] = [];
    const safeId = sanitizeCSSIdent(id);

    // Light mode
    lines.push(`[data-chart="${safeId}"] {`);
    for (const [key, value] of Object.entries(config)) {
      const safeKey = sanitizeCSSIdent(key);
      if (value.color) {
        lines.push(`  --color-${safeKey}: ${sanitizeCSSValue(value.color)};`);
      } else if (value.theme?.light) {
        lines.push(`  --color-${safeKey}: ${sanitizeCSSValue(value.theme.light)};`);
      }
    }
    lines.push('}');

    // Dark mode
    const darkEntries = Object.entries(config).filter(([, v]) => v.theme?.dark);
    if (darkEntries.length > 0) {
      lines.push(`.dark [data-chart="${safeId}"] {`);
      for (const [key, value] of darkEntries) {
        const safeKey = sanitizeCSSIdent(key);
        lines.push(`  --color-${safeKey}: ${sanitizeCSSValue(value.theme!.dark)};`);
      }
      lines.push('}');
    }

    return lines.join('\n');
  }
</script>

{@html `<style>${generateColorCSS()}</style>`}
