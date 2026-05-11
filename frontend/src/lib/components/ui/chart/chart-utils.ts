import { setContext } from 'svelte';

export type ChartConfig = Record<
  string,
  {
    label?: string;
    color?: string;
    theme?: { light: string; dark: string };
  }
>;

const CHART_CONTEXT_KEY = 'chart-context';

type ChartContext = {
  readonly config: ChartConfig;
};

export function setChartContext(ctx: ChartContext) {
  setContext(CHART_CONTEXT_KEY, ctx);
}
