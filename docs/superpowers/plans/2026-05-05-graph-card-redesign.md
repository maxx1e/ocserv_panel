# Graph Card Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign dashboard graph cards to match the svelteforge-admin-main visual reference with compact layout, dark mode support, and polished tooltips.

**Architecture:** Port shadcn-svelte Card primitives and ChartContainer from svelteforge reference. Add mode-watcher for dark/light theme toggle. Refactor MetricChart to use ChartContainer's LayerChart CSS overrides. Replace CardChart with Card.* primitives.

**Tech Stack:** SvelteKit 5, Tailwind CSS v4, LayerChart, mode-watcher, lucide-svelte, clsx + tailwind-merge

---

## File Structure

| Path | Responsibility |
|------|----------------|
| `frontend/src/lib/utils.ts` | CREATE — `cn()` class merge utility (clsx + tailwind-merge) |
| `frontend/src/lib/components/ui/card/card.svelte` | CREATE — Card.Root wrapper |
| `frontend/src/lib/components/ui/card/card-header.svelte` | CREATE — Card header layout |
| `frontend/src/lib/components/ui/card/card-content.svelte` | CREATE — Card content wrapper |
| `frontend/src/lib/components/ui/card/card-title.svelte` | CREATE — Title typography |
| `frontend/src/lib/components/ui/card/card-description.svelte` | CREATE — Description text |
| `frontend/src/lib/components/ui/card/index.ts` | CREATE — Re-exports |
| `frontend/src/lib/components/ui/chart/chart-container.svelte` | CREATE — LayerChart wrapper with CSS overrides |
| `frontend/src/lib/components/ui/chart/chart-tooltip.svelte` | CREATE — Custom tooltip |
| `frontend/src/lib/components/ui/chart/chart-style.svelte` | CREATE — Scoped theme CSS |
| `frontend/src/lib/components/ui/chart/chart-utils.ts` | CREATE — Config types + context |
| `frontend/src/lib/components/ui/chart/index.ts` | CREATE — Re-exports |
| `frontend/src/lib/components/ui/ThemeToggle.svelte` | CREATE — Dark/light toggle button |
| `frontend/src/lib/components/ui/MetricChart.svelte` | REWRITE — Use ChartContainer + AreaChart |
| `frontend/src/lib/components/ui/CardChart.svelte` | DELETE — Replaced by Card.* |
| `frontend/src/app.css` | MODIFY — Add CSS tokens, dark mode, remove old overrides |
| `frontend/src/lib/monitoring.ts` | MODIFY — Shorter X-axis, fewer ticks, chart config |
| `frontend/src/lib/components/ui/Navbar.svelte` | MODIFY — Add ThemeToggle |
| `frontend/src/routes/+layout.svelte` | MODIFY — Add ModeWatcher, dark class on body |
| `frontend/src/routes/+page.svelte` | MODIFY — Use Card.* for chart section, {#key mode.current} |
| `frontend/package.json` | MODIFY — Add dependencies |
| `frontend/vite.config.ts` | MODIFY — Add layerchart/svelte-ux to ssr.noExternal |

---

### Task 1: Install Dependencies

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/vite.config.ts`
- Create: `frontend/src/lib/utils.ts`

- [ ] **Step 1: Install npm packages**

```bash
cd /opt/ocserv_panel/frontend && npm install mode-watcher clsx tailwind-merge
```

- [ ] **Step 2: Add SSR noExternal for layerchart and svelte-ux**

In `frontend/vite.config.ts`, add `ssr` config so LayerChart renders server-side:

```typescript
import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';
import { svelteTesting } from '@testing-library/svelte/vite';

export default defineConfig({
  plugins: [tailwindcss(), sveltekit(), svelteTesting()],
  server: {
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: false
      }
    }
  },
  ssr: {
    noExternal: ['layerchart', 'svelte-ux']
  },
  test: {
    include: ['src/**/*.test.ts'],
    environment: 'jsdom',
    setupFiles: ['src/vitest.setup.ts'],
    globals: true
  }
});
```

- [ ] **Step 3: Create cn() utility**

Create `frontend/src/lib/utils.ts`:

```typescript
import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

- [ ] **Step 4: Verify build compiles**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-kit sync && npx svelte-check --tsconfig ./tsconfig.json
```

Expected: No errors.

- [ ] **Step 5: Commit**

```bash
cd /opt/ocserv_panel/frontend && git add package.json package-lock.json vite.config.ts src/lib/utils.ts && git commit -m "feat(frontend): add mode-watcher, clsx, tailwind-merge, cn() utility, SSR config"
```

---

### Task 2: CSS Token System and Dark Mode Variables

**Files:**
- Modify: `frontend/src/app.css`

- [ ] **Step 1: Replace app.css with token-based system**

Replace the entire contents of `frontend/src/app.css` with:

```css
@import 'tailwindcss';
@plugin '@tailwindcss/forms';

/* Scan LayerChart's internal classes so Tailwind v4 includes them */
@source "../node_modules/layerchart/**/*.svelte";
@source "../node_modules/svelte-ux/**/*.svelte";

@custom-variant dark (&:where(.dark, .dark *));

@theme {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--card-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-border: var(--border);
  --color-chart-1: var(--chart-1);
  --color-chart-2: var(--chart-2);
  --color-chart-3: var(--chart-3);
  --color-chart-4: var(--chart-4);
  --color-chart-5: var(--chart-5);
  /* Preserve existing slate palette */
  --color-slate-50: #f8fafc;
  --color-slate-100: #f1f5f9;
  --color-slate-200: #e2e8f0;
  --color-slate-300: #cbd5e1;
  --color-slate-400: #94a3b8;
  --color-slate-500: #64748b;
  --color-slate-600: #475569;
  --color-slate-700: #334155;
  --color-slate-800: #1e293b;
  --color-slate-900: #0f172a;
  --color-slate-950: #020617;
  --color-surface-content: #64748b;
}

:root {
  --background: oklch(0.985 0 0);
  --foreground: oklch(0.145 0 0);
  --card: oklch(1 0 0);
  --card-foreground: oklch(0.145 0 0);
  --muted: oklch(0.96 0 0);
  --muted-foreground: oklch(0.556 0 0);
  --border: oklch(0.922 0 0);
  --chart-1: oklch(0.585 0.233 277.117);
  --chart-2: oklch(0.68 0.162 149.48);
  --chart-3: oklch(0.715 0.143 215.221);
  --chart-4: oklch(0.627 0.265 303.9);
  --chart-5: oklch(0.769 0.188 85.394);
}

.dark {
  --background: oklch(0.145 0 0);
  --foreground: oklch(0.985 0 0);
  --card: oklch(0.205 0 0);
  --card-foreground: oklch(0.985 0 0);
  --muted: oklch(0.269 0 0);
  --muted-foreground: oklch(0.708 0 0);
  --border: oklch(0.35 0 0);
  --chart-1: oklch(0.673 0.245 277.117);
  --chart-2: oklch(0.74 0.17 149.48);
  --chart-3: oklch(0.77 0.15 215.221);
  --chart-4: oklch(0.7 0.27 303.9);
  --chart-5: oklch(0.82 0.2 85.394);
}
```

Note: The old `.grid`, `.rule`, `.tick` class overrides are removed — ChartContainer handles this now.

- [ ] **Step 2: Verify build**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-kit sync && npx svelte-check --tsconfig ./tsconfig.json
```

Expected: No errors (existing components still use explicit Tailwind classes which still resolve).

- [ ] **Step 3: Commit**

```bash
cd /opt/ocserv_panel/frontend && git add src/app.css && git commit -m "feat(frontend): add CSS token system with dark mode variables and chart palette"
```

---

### Task 3: Card Primitives

**Files:**
- Create: `frontend/src/lib/components/ui/card/card.svelte`
- Create: `frontend/src/lib/components/ui/card/card-header.svelte`
- Create: `frontend/src/lib/components/ui/card/card-content.svelte`
- Create: `frontend/src/lib/components/ui/card/card-title.svelte`
- Create: `frontend/src/lib/components/ui/card/card-description.svelte`
- Create: `frontend/src/lib/components/ui/card/index.ts`

- [ ] **Step 1: Create card.svelte**

Create `frontend/src/lib/components/ui/card/card.svelte`:

```svelte
<script lang="ts">
  import { cn } from '$lib/utils';
  import type { HTMLAttributes } from 'svelte/elements';
  import type { Snippet } from 'svelte';

  let {
    class: className,
    children,
    ...restProps
  }: HTMLAttributes<HTMLDivElement> & { children?: Snippet } = $props();
</script>

<div
  class={cn(
    "bg-card text-card-foreground flex flex-col gap-6 rounded-xl border border-border py-6 shadow-sm",
    className
  )}
  {...restProps}
>
  {#if children}
    {@render children()}
  {/if}
</div>
```

- [ ] **Step 2: Create card-header.svelte**

Create `frontend/src/lib/components/ui/card/card-header.svelte`:

```svelte
<script lang="ts">
  import { cn } from '$lib/utils';
  import type { HTMLAttributes } from 'svelte/elements';
  import type { Snippet } from 'svelte';

  let {
    class: className,
    children,
    ...restProps
  }: HTMLAttributes<HTMLDivElement> & { children?: Snippet } = $props();
</script>

<div
  class={cn(
    "flex items-center justify-between gap-2 px-6",
    className
  )}
  {...restProps}
>
  {#if children}
    {@render children()}
  {/if}
</div>
```

- [ ] **Step 3: Create card-content.svelte**

Create `frontend/src/lib/components/ui/card/card-content.svelte`:

```svelte
<script lang="ts">
  import { cn } from '$lib/utils';
  import type { HTMLAttributes } from 'svelte/elements';
  import type { Snippet } from 'svelte';

  let {
    class: className,
    children,
    ...restProps
  }: HTMLAttributes<HTMLDivElement> & { children?: Snippet } = $props();
</script>

<div class={cn("px-6", className)} {...restProps}>
  {#if children}
    {@render children()}
  {/if}
</div>
```

- [ ] **Step 4: Create card-title.svelte**

Create `frontend/src/lib/components/ui/card/card-title.svelte`:

```svelte
<script lang="ts">
  import { cn } from '$lib/utils';
  import type { HTMLAttributes } from 'svelte/elements';
  import type { Snippet } from 'svelte';

  let {
    class: className,
    children,
    ...restProps
  }: HTMLAttributes<HTMLHeadingElement> & { children?: Snippet } = $props();
</script>

<h3 class={cn("text-sm font-semibold leading-none", className)} {...restProps}>
  {#if children}
    {@render children()}
  {/if}
</h3>
```

- [ ] **Step 5: Create card-description.svelte**

Create `frontend/src/lib/components/ui/card/card-description.svelte`:

```svelte
<script lang="ts">
  import { cn } from '$lib/utils';
  import type { HTMLAttributes } from 'svelte/elements';
  import type { Snippet } from 'svelte';

  let {
    class: className,
    children,
    ...restProps
  }: HTMLAttributes<HTMLParagraphElement> & { children?: Snippet } = $props();
</script>

<p class={cn("text-muted-foreground text-sm", className)} {...restProps}>
  {#if children}
    {@render children()}
  {/if}
</p>
```

- [ ] **Step 6: Create index.ts**

Create `frontend/src/lib/components/ui/card/index.ts`:

```typescript
import Root from "./card.svelte";
import Header from "./card-header.svelte";
import Content from "./card-content.svelte";
import Title from "./card-title.svelte";
import Description from "./card-description.svelte";

export {
  Root,
  Header,
  Content,
  Title,
  Description,
  Root as Card,
  Header as CardHeader,
  Content as CardContent,
  Title as CardTitle,
  Description as CardDescription,
};
```

- [ ] **Step 7: Verify build**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-kit sync && npx svelte-check --tsconfig ./tsconfig.json
```

Expected: No errors.

- [ ] **Step 8: Commit**

```bash
cd /opt/ocserv_panel/frontend && git add src/lib/components/ui/card/ && git commit -m "feat(frontend): add shadcn-svelte Card primitives (Root, Header, Content, Title, Description)"
```

---

### Task 4: Chart System Components

**Files:**
- Create: `frontend/src/lib/components/ui/chart/chart-utils.ts`
- Create: `frontend/src/lib/components/ui/chart/chart-style.svelte`
- Create: `frontend/src/lib/components/ui/chart/chart-container.svelte`
- Create: `frontend/src/lib/components/ui/chart/chart-tooltip.svelte`
- Create: `frontend/src/lib/components/ui/chart/index.ts`

- [ ] **Step 1: Create chart-utils.ts**

Create `frontend/src/lib/components/ui/chart/chart-utils.ts`:

```typescript
import { getContext, setContext } from 'svelte';

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

export function useChart(): ChartContext {
  return getContext<ChartContext>(CHART_CONTEXT_KEY);
}
```

- [ ] **Step 2: Create chart-style.svelte**

Create `frontend/src/lib/components/ui/chart/chart-style.svelte`:

```svelte
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
```

- [ ] **Step 3: Create chart-container.svelte**

Create `frontend/src/lib/components/ui/chart/chart-container.svelte`:

```svelte
<script lang="ts">
  import { cn } from '$lib/utils';
  import type { HTMLAttributes } from 'svelte/elements';
  import type { Snippet } from 'svelte';
  import ChartStyle from './chart-style.svelte';
  import { setChartContext, type ChartConfig } from './chart-utils';

  const uid = crypto.randomUUID().slice(0, 8);

  let {
    id,
    class: className,
    children,
    config,
    ...restProps
  }: HTMLAttributes<HTMLDivElement> & {
    config: ChartConfig;
    children?: Snippet;
    id?: string;
  } = $props();

  const chartId = `chart-${id || uid}`;

  setChartContext({
    get config() {
      return config;
    },
  });
</script>

<div
  data-chart={chartId}
  class={cn(
    "flex justify-center overflow-visible text-xs",
    // Hide highlight stroke on points
    "[&_.lc-highlight-point]:stroke-transparent",
    // Override default line stroke color
    "[&_.lc-line]:stroke-border/50",
    // Hide the vertical highlight line on hover
    "[&_.lc-highlight-line]:stroke-0",
    // Keep area paths fully opaque (no fade on hover)
    "[&_.lc-area-path]:opacity-100 [&_.lc-highlight-line]:opacity-100 [&_.lc-highlight-point]:opacity-100 [&_.lc-spline-path]:opacity-100",
    // Text sizing
    "[&_.lc-text]:text-xs [&_.lc-text-svg]:overflow-visible",
    // Hide tick marks between axis labels and chart
    "[&_.lc-axis-tick]:stroke-0",
    // Hide axis rules (grid line already there)
    "[&_.lc-rule-x-line:not(.lc-grid-x-rule)]:stroke-0 [&_.lc-rule-y-line:not(.lc-grid-y-rule)]:stroke-0",
    // Grid line colors using theme token
    "[&_.lc-grid-x-radial-line]:stroke-border [&_.lc-grid-x-radial-circle]:stroke-border",
    "[&_.lc-grid-y-radial-line]:stroke-border [&_.lc-grid-y-radial-circle]:stroke-border",
    // Tick label styling
    "[&_.lc-axis-tick-label]:fill-muted-foreground [&_.lc-axis-tick-label]:font-normal",
    // Make root container full width
    "[&_.lc-root-container]:w-full",
    // Transparent fills for layout/tooltip rects
    "[&_.lc-tooltip-rects-g]:fill-transparent",
    "[&_.lc-layout-svg-g]:fill-transparent",
    // Labels
    "[&_.lc-labels-text:not([fill])]:fill-foreground [&_text]:stroke-transparent",
    className
  )}
  {...restProps}
>
  <ChartStyle id={chartId} {config} />
  {#if children}
    {@render children()}
  {/if}
</div>
```

- [ ] **Step 4: Create chart-tooltip.svelte**

Create `frontend/src/lib/components/ui/chart/chart-tooltip.svelte`:

```svelte
<script lang="ts">
  import { cn } from '$lib/utils';
  import type { Snippet } from 'svelte';
  import { Tooltip as TooltipPrimitive } from 'layerchart';

  type Props = {
    class?: string;
    label?: string;
    valueFormatter?: (value: number) => string;
    children?: Snippet<[{ data: any }]>;
  };

  let { class: className, label, valueFormatter, children }: Props = $props();
</script>

<TooltipPrimitive.Root variant="none">
  {#snippet children({ data })}
    <div
      class={cn(
        "border-border/50 bg-card text-card-foreground grid min-w-[9rem] items-start gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs shadow-xl",
        className
      )}
    >
      {#if children}
        {@render children({ data })}
      {:else if data}
        {#if data.timestamp}
          <div class="font-medium">
            {data.timestamp instanceof Date
              ? data.timestamp.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
              : String(data.timestamp)}
          </div>
        {/if}
        <div class="flex items-center justify-between gap-4">
          <span class="text-muted-foreground">{label ?? 'Value'}</span>
          <span class="font-mono font-medium tabular-nums">
            {valueFormatter && typeof data.value === 'number'
              ? valueFormatter(data.value)
              : data.value?.toLocaleString() ?? '--'}
          </span>
        </div>
      {/if}
    </div>
  {/snippet}
</TooltipPrimitive.Root>
```

- [ ] **Step 5: Create index.ts**

Create `frontend/src/lib/components/ui/chart/index.ts`:

```typescript
import Container from "./chart-container.svelte";
import Tooltip from "./chart-tooltip.svelte";
import Style from "./chart-style.svelte";

export { Container, Tooltip, Style };
export { type ChartConfig, setChartContext, useChart } from "./chart-utils";
```

- [ ] **Step 6: Verify build**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-kit sync && npx svelte-check --tsconfig ./tsconfig.json
```

Expected: No errors.

- [ ] **Step 7: Commit**

```bash
cd /opt/ocserv_panel/frontend && git add src/lib/components/ui/chart/ && git commit -m "feat(frontend): add ChartContainer, ChartTooltip, and ChartStyle components"
```

---

### Task 5: ThemeToggle and Dark Mode Wiring

**Files:**
- Create: `frontend/src/lib/components/ui/ThemeToggle.svelte`
- Modify: `frontend/src/lib/components/ui/Navbar.svelte`
- Modify: `frontend/src/routes/+layout.svelte`

- [ ] **Step 1: Create ThemeToggle.svelte**

Create `frontend/src/lib/components/ui/ThemeToggle.svelte`:

```svelte
<script lang="ts">
  import { toggleMode, mode } from 'mode-watcher';
  import { Sun, Moon } from 'lucide-svelte';
</script>

<button
  type="button"
  onclick={toggleMode}
  class="inline-flex items-center justify-center rounded-md p-2 text-slate-500 hover:bg-slate-100 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200 transition-colors"
  aria-label={mode.current === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
>
  {#if mode.current === 'dark'}
    <Sun class="h-5 w-5" />
  {:else}
    <Moon class="h-5 w-5" />
  {/if}
</button>
```

- [ ] **Step 2: Add ThemeToggle to Navbar**

Replace the entire content of `frontend/src/lib/components/ui/Navbar.svelte` with:

```svelte
<script lang="ts">
  import { User } from 'lucide-svelte';
  import ThemeToggle from './ThemeToggle.svelte';

  type Props = {
    title: string;
    username: string;
    onLogout: () => void;
  };

  let { title, username, onLogout }: Props = $props();
  let showDropdown = $state(false);

  function toggleDropdown() {
    showDropdown = !showDropdown;
  }

  function handleLogout() {
    showDropdown = false;
    onLogout();
  }
</script>

<header class="h-16 bg-card border-b border-border flex items-center justify-between px-6 sticky top-0 z-20">
  <h2 class="text-lg font-semibold text-foreground">{title}</h2>

  <div class="flex items-center gap-2">
    <ThemeToggle />

    <div class="relative">
      <button
        type="button"
        onclick={toggleDropdown}
        class="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        <div class="w-8 h-8 rounded-full bg-muted flex items-center justify-center">
          <User class="w-4 h-4 text-muted-foreground" />
        </div>
        <span class="hidden md:inline">{username}</span>
      </button>

      {#if showDropdown}
        <div class="absolute right-0 mt-2 w-48 bg-card rounded-lg shadow-lg border border-border py-1 z-50">
          <button
            type="button"
            onclick={handleLogout}
            class="w-full text-left px-4 py-2 text-sm text-card-foreground hover:bg-muted"
          >
            Sign out
          </button>
        </div>
      {/if}
    </div>
  </div>
</header>
```

- [ ] **Step 3: Wire ModeWatcher in layout**

Replace the entire content of `frontend/src/routes/+layout.svelte` with:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { ModeWatcher } from 'mode-watcher';
  import '../app.css';
  import { logout } from '$lib/api';
  import { createAuthState, refreshAuthState } from '$lib/auth';
  import { navItems } from '$lib/navigation';
  import Sidebar from '$lib/components/ui/Sidebar.svelte';
  import Navbar from '$lib/components/ui/Navbar.svelte';
  import { page } from '$app/stores';

  let { children } = $props();
  let auth = $state(createAuthState());
  let logoutError = $state('');

  const visibleNavItems = $derived(
    auth.principal
      ? navItems.filter((item) => !item.roles || item.roles.some((role) => auth.principal?.roles?.includes(role)))
      : []
  );

  const pageTitle = $derived(
    auth.setupRequired ? 'Setup' :
    !auth.principal ? 'Login' :
    visibleNavItems.find(i => i.href === $page.url.pathname || (i.href !== '/' && $page.url.pathname.startsWith(i.href)))?.label ?? 'Dashboard'
  );

  onMount(() => {
    void refreshAuthState(auth);
  });

  async function handleLogout() {
    logoutError = '';
    try {
      await logout();
      auth.principal = null;
      window.location.href = '/login';
    } catch (error) {
      logoutError = error instanceof Error ? error.message : 'Logout failed.';
    }
  }
</script>

<ModeWatcher />

<svelte:head>
  <title>ocserv Panel</title>
  <meta name="description" content="Local-first web administration panel for a single ocserv server" />
</svelte:head>

{#if auth.setupRequired || !auth.principal}
  <!-- No sidebar for login/setup pages -->
  <div class="min-h-screen bg-background text-foreground">
    {@render children()}
  </div>
{:else}
  <div class="min-h-screen bg-background text-foreground">
    <Sidebar
      navItems={visibleNavItems}
      username={auth.principal?.username ?? ''}
      roles={auth.principal?.roles ?? []}
      onLogout={handleLogout}
      loading={auth.loading}
      setupRequired={auth.setupRequired ?? false}
      error={auth.error || logoutError}
    />
    <div class="md:ml-64">
      <Navbar
        title={pageTitle}
        username={auth.principal?.username ?? ''}
        onLogout={handleLogout}
      />
      <main class="p-6">
        {@render children()}
      </main>
    </div>
  </div>
{/if}
```

- [ ] **Step 4: Verify build**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-kit sync && npx svelte-check --tsconfig ./tsconfig.json
```

Expected: No errors.

- [ ] **Step 5: Commit**

```bash
cd /opt/ocserv_panel/frontend && git add src/lib/components/ui/ThemeToggle.svelte src/lib/components/ui/Navbar.svelte src/routes/+layout.svelte && git commit -m "feat(frontend): add dark mode toggle with mode-watcher in navbar"
```

---

### Task 6: Update monitoring.ts for Compact Charts

**Files:**
- Modify: `frontend/src/lib/monitoring.ts`

- [ ] **Step 1: Update formatXAxisTime for shorter labels**

In `frontend/src/lib/monitoring.ts`, replace the `formatXAxisTime` function with a shorter format that only shows `HH:MM`:

```typescript
/**
 * Formats a Date for display on the chart X-axis.
 * Short format "HH:MM" for compact 160px charts.
 */
export function formatXAxisTime(d: Date | number): string {
  const date = d instanceof Date ? d : new Date(d);
  const hours = date.getHours().toString().padStart(2, '0');
  const minutes = date.getMinutes().toString().padStart(2, '0');
  return `${hours}:${minutes}`;
}
```

- [ ] **Step 2: Reduce tick count for compact charts**

Replace the `getXTickCount` function with a lower-cap version:

```typescript
/**
 * Returns the number of ticks for the X (time) axis.
 * Capped at 4-5 to prevent overlap in compact 160px-height charts.
 */
export function getXTickCount(dataLength: number): number {
  if (dataLength <= 30) return Math.min(dataLength, 3);
  if (dataLength <= 60) return 4;
  return 5;
}
```

- [ ] **Step 3: Add getChartColorVar helper for CSS variable chart config**

Add this new export at the end of `frontend/src/lib/monitoring.ts`:

```typescript
/**
 * Returns the CSS variable color string for a metric, suitable for
 * use in ChartConfig color fields.
 */
export function getChartColorVar(metric: string): string {
  const map: Record<string, string> = {
    host_cpu_utilization: 'var(--chart-1)',
    host_memory_utilization: 'var(--chart-1)',
    net_interface_rx_bytes: 'var(--chart-2)',
    net_interface_tx_bytes: 'var(--chart-3)',
    vpn_interface_rx_bytes: 'var(--chart-2)',
    vpn_interface_tx_bytes: 'var(--chart-3)',
    user_rx_bytes: 'var(--chart-2)',
    user_tx_bytes: 'var(--chart-3)',
    active_sessions: 'var(--chart-4)',
    concurrent_sessions: 'var(--chart-4)',
    connection_rate: 'var(--chart-5)',
  };
  return map[metric] ?? 'var(--chart-1)';
}
```

- [ ] **Step 4: Verify build**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-kit sync && npx svelte-check --tsconfig ./tsconfig.json
```

Expected: No errors.

- [ ] **Step 5: Commit**

```bash
cd /opt/ocserv_panel/frontend && git add src/lib/monitoring.ts && git commit -m "feat(frontend): compact x-axis format (HH:MM), fewer ticks, chart color CSS vars"
```

---

### Task 7: Rewrite MetricChart to Use ChartContainer

**Files:**
- Rewrite: `frontend/src/lib/components/ui/MetricChart.svelte`

- [ ] **Step 1: Rewrite MetricChart.svelte**

Replace the entire contents of `frontend/src/lib/components/ui/MetricChart.svelte` with:

```svelte
<script lang="ts">
  import { Area, Axis, Chart, Highlight, LinearGradient, Svg, Tooltip } from 'layerchart';
  import * as ChartUI from '$lib/components/ui/chart/index';
  import {
    formatMetricValue,
    formatXAxisTime,
    getChartClasses,
    getChartConfig,
    getChartColorVar,
    getXTickCount,
  } from '$lib/monitoring';
  import type { ChartConfig } from '$lib/components/ui/chart/chart-utils';

  type TimeData = { timestamp: Date; value: number };

  type Props = {
    /** Sorted array of time-value pairs. Empty array -> placeholder shown. */
    data: TimeData[];
    /** Backend metric name used to derive colours, domain, and axis formatting. */
    metric: string;
    /** Pixel height of the chart container. Defaults to 160. */
    height?: number;
  };

  let { data, metric, height = 160 }: Props = $props();

  const config = $derived(getChartConfig(metric));
  const classes = $derived(getChartClasses(metric));
  const xTickCount = $derived(getXTickCount(data.length));
  const chartColor = $derived(getChartColorVar(metric));

  const chartConfig: ChartConfig = $derived({
    value: { label: metric, color: chartColor },
  });

  function tooltipFormatter(value: number): string {
    return formatMetricValue(metric, value);
  }
</script>

<div style="height: {height}px" class="w-full">
  {#if data.length > 0}
    <ChartUI.Container config={chartConfig} class="h-full w-full">
      <Chart
        {data}
        x="timestamp"
        y="value"
        yDomain={config.yDomain}
        yNice={config.yNice}
        padding={{ left: 44, bottom: 24, top: 8, right: 8 }}
        tooltip={{ mode: 'bisect-x' }}
      >
        <Svg>
          <Axis
            placement="left"
            grid
            format={config.yAxisFormat}
          />
          <Axis
            placement="bottom"
            format={(d: Date | number) => formatXAxisTime(d)}
            ticks={xTickCount}
          />
          <LinearGradient
            stops={[classes.gradientTop, classes.gradientBottom]}
            vertical
          >
            {#snippet children({ gradient }: { gradient: string })}
              <Area fill={gradient} line={{ class: classes.stroke }} />
            {/snippet}
          </LinearGradient>
          <Highlight points lines />
        </Svg>
        <ChartUI.Tooltip
          label={metric.replace(/_/g, ' ')}
          valueFormatter={tooltipFormatter}
        />
      </Chart>
    </ChartUI.Container>
  {:else}
    <div class="h-full flex items-center justify-center rounded-lg border border-dashed border-border bg-muted/30">
      <p class="text-sm text-muted-foreground">No data available</p>
    </div>
  {/if}
</div>
```

Key changes from original:
- Default height reduced to 160px
- Wrapped in `ChartUI.Container` (provides all LayerChart CSS overrides)
- Reduced padding: `left: 44, bottom: 24, top: 8, right: 8` (more compact)
- Removed `rule` from Axis components (ChartContainer hides them)
- Uses `ChartUI.Tooltip` instead of inline tooltip rendering
- Empty state uses theme tokens (`border-border`, `bg-muted/30`, `text-muted-foreground`)

- [ ] **Step 2: Verify build**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-kit sync && npx svelte-check --tsconfig ./tsconfig.json
```

Expected: No errors.

- [ ] **Step 3: Commit**

```bash
cd /opt/ocserv_panel/frontend && git add src/lib/components/ui/MetricChart.svelte && git commit -m "feat(frontend): rewrite MetricChart to use ChartContainer with compact 160px layout"
```

---

### Task 8: Update Dashboard Page to Use Card Primitives

**Files:**
- Modify: `frontend/src/routes/+page.svelte`
- Delete: `frontend/src/lib/components/ui/CardChart.svelte`

- [ ] **Step 1: Update imports in +page.svelte**

In `frontend/src/routes/+page.svelte`, replace the import section (lines 1-35 approximately). Change:

```
import CardChart from '$lib/components/ui/CardChart.svelte';
```

to:

```
import * as Card from '$lib/components/ui/card/index';
import { mode } from 'mode-watcher';
```

Make sure `CardChart` import is removed entirely and `Card` + `mode` are imported.

- [ ] **Step 2: Replace chart grid rendering**

In `frontend/src/routes/+page.svelte`, find the chart grid section (the block that starts with `<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">` inside the monitoring section) and replace it with:

```svelte
      {#key mode.current}
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {#each chartDefinitions as chart}
            {#if shouldShowChart(chart.metric)}
              <Card.Root>
                <Card.Header>
                  <div>
                    <Card.Title>{chart.title}</Card.Title>
                    <Card.Description>Last {monitoringWindow}</Card.Description>
                  </div>
                  <span class="text-sm font-semibold text-foreground tabular-nums">
                    {metricValue(chart.metric)}
                  </span>
                </Card.Header>
                <Card.Content>
                  <MetricChart data={chartData(chart.metric)} metric={chart.metric} height={160} />
                </Card.Content>
              </Card.Root>
            {/if}
          {/each}
        </div>
      {/key}
```

Note: `gap-6` reduced to `gap-4` for tighter grid. `{#key mode.current}` forces chart re-render on theme change.

- [ ] **Step 3: Delete CardChart.svelte**

```bash
rm /opt/ocserv_panel/frontend/src/lib/components/ui/CardChart.svelte
```

- [ ] **Step 4: Update history page to remove CardChart dependency**

The history page at `frontend/src/routes/history/+page.svelte` also imports `CardChart`. Replace the import:

```
- import CardChart from '$lib/components/ui/CardChart.svelte';
+ import * as Card from '$lib/components/ui/card/index';
+ import { mode } from 'mode-watcher';
```

Then replace each `<CardChart title={chart.title} chartHeight={200}>...</CardChart>` usage with:

```svelte
<Card.Root>
  <Card.Header>
    <div>
      <Card.Title>{chart.title}</Card.Title>
      <Card.Description>Last {selectedWindow}</Card.Description>
    </div>
    <span class="text-sm font-semibold text-foreground tabular-nums">
      {metricValue(chart.metric)}
    </span>
  </Card.Header>
  <Card.Content>
    <MetricChart data={chartData(chart.metric)} metric={chart.metric} height={160} />
  </Card.Content>
</Card.Root>
```

Wrap the chart grid in `{#key mode.current}` for dark-mode re-rendering. Adjust the variable names (`selectedWindow`, `metricValue`, `chartData`) to match whatever exists in the history page.

- [ ] **Step 5: Remove CardChart.svelte file**

```bash
rm /opt/ocserv_panel/frontend/src/lib/components/ui/CardChart.svelte
```

- [ ] **Step 6: Verify no remaining references**

```bash
grep -r "CardChart" /opt/ocserv_panel/frontend/src/ --include="*.svelte" --include="*.ts"
```

Expected: No results.

- [ ] **Step 5: Verify build**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-kit sync && npx svelte-check --tsconfig ./tsconfig.json
```

Expected: No errors.

- [ ] **Step 6: Commit**

```bash
cd /opt/ocserv_panel/frontend && git add -A && git commit -m "feat(frontend): replace CardChart with Card primitives in dashboard, add mode-aware chart grid"
```

---

### Task 9: Loading Skeleton for Charts

**Files:**
- Modify: `frontend/src/routes/+page.svelte`

- [ ] **Step 1: Add skeleton loading state for monitoring charts**

In `frontend/src/routes/+page.svelte`, replace the existing monitoring loading spinner (the block `{#if monitoringLoading && !monitoring}`) with a card-based skeleton:

```svelte
    {#if monitoringLoading && !monitoring}
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {#each Array(4) as _}
          <Card.Root>
            <Card.Header>
              <div>
                <div class="h-4 w-24 rounded bg-muted animate-pulse"></div>
                <div class="h-3 w-16 rounded bg-muted animate-pulse mt-1.5"></div>
              </div>
              <div class="h-5 w-12 rounded bg-muted animate-pulse"></div>
            </Card.Header>
            <Card.Content>
              <div class="h-[160px] w-full rounded-lg bg-muted/30 animate-pulse flex items-end gap-1 px-4 pb-4">
                {#each [40, 55, 35, 70, 50, 65, 45, 60, 75, 50, 80, 55] as h}
                  <div class="flex-1 rounded-sm bg-muted" style="height: {h}%"></div>
                {/each}
              </div>
            </Card.Content>
          </Card.Root>
        {/each}
      </div>
    {:else}
```

This produces 4 skeleton cards in a 2-column grid with bar-shaped placeholders suggesting chart data.

- [ ] **Step 2: Verify build**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-kit sync && npx svelte-check --tsconfig ./tsconfig.json
```

Expected: No errors.

- [ ] **Step 3: Commit**

```bash
cd /opt/ocserv_panel/frontend && git add src/routes/+page.svelte && git commit -m "feat(frontend): add card-based skeleton loading state for monitoring charts"
```

---

### Task 10: Final Verification and Cleanup

**Files:**
- Verify all modified files

- [ ] **Step 1: Full type check**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-kit sync && npx svelte-check --tsconfig ./tsconfig.json
```

Expected: 0 errors.

- [ ] **Step 2: Run existing tests**

```bash
cd /opt/ocserv_panel/frontend && npm run test
```

Expected: All existing tests pass (or no tests exist to fail).

- [ ] **Step 3: Build production bundle**

```bash
cd /opt/ocserv_panel/frontend && npm run build
```

Expected: Build succeeds without errors.

- [ ] **Step 4: Check for unused imports or dead code**

```bash
grep -r "CardChart" /opt/ocserv_panel/frontend/src/ --include="*.svelte" --include="*.ts"
```

Expected: No results (CardChart fully removed).

```bash
grep -r "from '\$lib/components/ui/CardChart" /opt/ocserv_panel/frontend/src/
```

Expected: No results.

- [ ] **Step 5: Verify git status is clean**

```bash
cd /opt/ocserv_panel/frontend && git status
```

Expected: Working tree clean (all changes committed in previous tasks).

---

## Summary of Commits

1. `feat(frontend): add mode-watcher, clsx, tailwind-merge, cn() utility, SSR config`
2. `feat(frontend): add CSS token system with dark mode variables and chart palette`
3. `feat(frontend): add shadcn-svelte Card primitives (Root, Header, Content, Title, Description)`
4. `feat(frontend): add ChartContainer, ChartTooltip, and ChartStyle components`
5. `feat(frontend): add dark mode toggle with mode-watcher in navbar`
6. `feat(frontend): compact x-axis format (HH:MM), fewer ticks, chart color CSS vars`
7. `feat(frontend): rewrite MetricChart to use ChartContainer with compact 160px layout`
8. `feat(frontend): replace CardChart with Card primitives in dashboard, add mode-aware chart grid`
9. `feat(frontend): add card-based skeleton loading state for monitoring charts`
