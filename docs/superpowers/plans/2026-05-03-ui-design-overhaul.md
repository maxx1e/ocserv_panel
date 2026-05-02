# UI Design Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the ocserv panel frontend with Tailwind CSS, Lucide icons, and LayerChart, adopting the notus-svelte reference design language with a slate theme.

**Architecture:** Full page rebuild approach — each page is rebuilt from scratch using Tailwind utility classes and new shared components. The layout shell (sidebar + navbar) is rebuilt first, then pages one at a time. All existing functionality and API contracts are preserved; only the visual layer changes.

**Tech Stack:** SvelteKit (existing), Tailwind CSS v4, lucide-svelte, layerchart, @tailwindcss/forms

---

## File Structure

### New files to create:
- `frontend/tailwind.config.js` — Tailwind configuration with slate theme
- `frontend/postcss.config.js` — PostCSS config for Tailwind
- `frontend/src/app.css` — Replaced with Tailwind directives + minimal custom styles
- `frontend/src/lib/components/ui/CardStats.svelte` — Stat card with icon
- `frontend/src/lib/components/ui/CardChart.svelte` — Chart card wrapper for LayerChart
- `frontend/src/lib/components/ui/CardTable.svelte` — Table card wrapper
- `frontend/src/lib/components/ui/CardSettings.svelte` — Settings form card
- `frontend/src/lib/components/ui/EmptyState.svelte` — Empty/placeholder state
- `frontend/src/lib/components/ui/Navbar.svelte` — Top navbar component
- `frontend/src/lib/components/ui/Sidebar.svelte` — Slate sidebar component
- `frontend/src/lib/components/ui/UserDropdown.svelte` — User dropdown in navbar

### Files to modify:
- `frontend/package.json` — Add tailwindcss, @tailwindcss/forms, lucide-svelte, layerchart, autoprefixer, postcss
- `frontend/svelte.config.js` — Add postcss plugin
- `frontend/vite.config.ts` — No changes needed (Vite handles PostCSS via config)
- `frontend/src/app.html` — Add body classes
- `frontend/src/routes/+layout.svelte` — Rebuild with sidebar + navbar
- `frontend/src/routes/+page.svelte` — Rebuild dashboard
- `frontend/src/routes/login/+page.svelte` — Rebuild login
- `frontend/src/routes/setup/+page.svelte` — Rebuild setup
- `frontend/src/routes/settings/+page.svelte` — Rebuild settings
- `frontend/src/routes/history/+page.svelte` — Rebuild history
- `frontend/src/routes/users/+page.svelte` — Rebuild placeholder
- `frontend/src/routes/sessions/+page.svelte` — Rebuild placeholder
- `frontend/src/routes/config/+page.svelte` — Rebuild placeholder
- `frontend/src/routes/certificates/+page.svelte` — Rebuild placeholder
- `frontend/src/routes/audit/+page.svelte` — Rebuild placeholder
- `frontend/src/lib/navigation.ts` — Add icons to nav items

### Files to remove (after replacements are complete):
- `frontend/src/lib/components/monitoring/MetricCard.svelte` — Replaced by CardStats
- `frontend/src/lib/components/monitoring/TimeSeriesChart.svelte` — Replaced by CardChart + LayerChart
- `frontend/src/lib/components/monitoring/AnalyticsCard.svelte` — Replaced by CardStats
- `frontend/src/lib/components/monitoring/WindowSelector.svelte` — Rebuilt with Tailwind
- `frontend/src/lib/components/monitoring/UnavailableMetric.svelte` — Rebuilt with Tailwind
- `frontend/src/lib/components/settings/SettingsNav.svelte` — Rebuilt with Tailwind
- `frontend/src/lib/components/settings/RestartBanner.svelte` — Rebuilt with Tailwind
- `frontend/src/lib/components/settings/SettingsSection.svelte` — Replaced by CardSettings
- `frontend/src/lib/components/settings/SettingField.svelte` — Rebuilt with Tailwind
- `frontend/src/lib/components/settings/SettingToggle.svelte` — Rebuilt with Tailwind
- `frontend/src/lib/PlaceholderPage.svelte` — Replaced by EmptyState

### Files preserved unchanged:
- `frontend/src/lib/auth.ts` — No changes
- `frontend/src/lib/api.ts` — No changes
- `frontend/src/lib/monitoring.ts` — No changes

---

## Task 1: Install Dependencies and Configure Tailwind

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/tailwind.config.js`
- Create: `frontend/postcss.config.js`
- Modify: `frontend/svelte.config.js`

- [ ] **Step 1: Install npm dependencies**

```bash
cd frontend && npm install -D tailwindcss@next @tailwindcss/forms@next autoprefixer postcss && npm install lucide-svelte layerchart
```

Run: `cd frontend && npm install`
Expected: Dependencies installed successfully

- [ ] **Step 2: Create tailwind.config.js**

```js
// frontend/tailwind.config.js
/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
        slate: {
          50: '#f8fafc',
          100: '#f1f5f9',
          200: '#e2e8f0',
          300: '#cbd5e1',
          400: '#94a3b8',
          500: '#64748b',
          600: '#475569',
          700: '#334155',
          800: '#1e293b',
          900: '#0f172a',
          950: '#020617',
        }
      }
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
  ],
}
```

- [ ] **Step 3: Create postcss.config.js**

```js
// frontend/postcss.config.js
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
```

- [ ] **Step 4: Update svelte.config.js to use PostCSS**

```js
// frontend/svelte.config.js
import adapter from '@sveltejs/adapter-auto';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter()
  }
};

export default config;
```

Note: `vitePreprocess()` already handles PostCSS. No changes needed if it's already there. Verify it's present.

- [ ] **Step 5: Verify Tailwind is working**

Replace `frontend/src/app.css` with Tailwind directives:

```css
/* frontend/src/app.css */
@tailwind base;
@tailwind components;
@tailwind utilities;

/* Minimal custom styles that don't map to Tailwind utilities */
```

Run: `cd frontend && npm run dev -- --port 5173 &`
Expected: Dev server starts without errors. Check that Tailwind classes work by adding a test class temporarily.

- [ ] **Step 6: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/tailwind.config.js frontend/postcss.config.js frontend/svelte.config.js frontend/src/app.css
git commit -m "feat(ui): install Tailwind CSS v4, lucide-svelte, and layerchart dependencies"
```

---

## Task 2: Create Shared UI Components

**Files:**
- Create: `frontend/src/lib/components/ui/CardStats.svelte`
- Create: `frontend/src/lib/components/ui/CardChart.svelte`
- Create: `frontend/src/lib/components/ui/CardTable.svelte`
- Create: `frontend/src/lib/components/ui/CardSettings.svelte`
- Create: `frontend/src/lib/components/ui/EmptyState.svelte`

- [ ] **Step 1: Create CardStats component**

```svelte
<!-- frontend/src/lib/components/ui/CardStats.svelte -->
<script lang="ts">
  import type { Component } from 'svelte';
  import type { Snippet } from 'svelte';

  type Props = {
    title: string;
    value: string;
    subtitle?: string;
    icon?: Component;
    trend?: { direction: 'up' | 'down'; value: string };
    unavailable?: boolean;
    hint?: string;
  };

  let { title, value, subtitle, icon, trend, unavailable = false, hint }: Props = $props();
</script>

<article
  class="bg-white rounded-lg shadow-sm p-6 {unavailable ? 'bg-slate-50 border border-dashed border-slate-300' : ''}"
  aria-live="polite"
>
  <div class="flex items-center justify-between">
    <div class="flex-1 min-w-0">
      <p class="text-sm font-semibold text-slate-500 uppercase tracking-wide">{title}</p>
      <p class="mt-1 text-2xl font-bold {unavailable ? 'text-slate-400' : 'text-slate-700'} truncate">{value}</p>
      {#if trend}
        <p class="mt-1 text-sm">
          <span class={trend.direction === 'up' ? 'text-emerald-600' : 'text-red-600'}>
            {trend.direction === 'up' ? '↑' : '↓'} {trend.value}
          </span>
        </p>
      {/if}
      {#if subtitle}
        <p class="mt-1 text-sm text-slate-500">{subtitle}</p>
      {/if}
      {#if hint && unavailable}
        <p class="mt-1 text-xs text-slate-400">{hint}</p>
      {/if}
    </div>
    {#if icon}
      <div class="flex-shrink-0 ml-4">
        <div class="w-12 h-12 rounded-full bg-slate-100 flex items-center justify-center">
          <svelte:component this={icon} class="w-6 h-6 text-slate-600" />
        </div>
      </div>
    {/if}
  </div>
</article>
```

- [ ] **Step 2: Create CardChart component**

```svelte
<!-- frontend/src/lib/components/ui/CardChart.svelte -->
<script lang="ts">
  type Props = {
    title: string;
    subtitle?: string;
    latestValue?: string;
    children: import('svelte').Snippet;
    actions?: import('svelte').Snippet;
  };

  let { title, subtitle, latestValue, children, actions }: Props = $props();
</script>

<div class="bg-white rounded-lg shadow-sm">
  <div class="flex items-center justify-between px-6 py-4 border-b border-slate-100">
    <div>
      <h3 class="text-sm font-semibold text-slate-700">{title}</h3>
      {#if subtitle}
        <p class="text-xs text-slate-500 mt-0.5">{subtitle}</p>
      {/if}
    </div>
    <div class="flex items-center gap-3">
      {#if latestValue}
        <span class="text-sm font-semibold text-slate-700">{latestValue}</span>
      {/if}
      {#if actions}
        {@render actions()}
      {/if}
    </div>
  </div>
  <div class="p-4">
    {@render children()}
  </div>
</div>
```

- [ ] **Step 3: Create CardTable component**

```svelte
<!-- frontend/src/lib/components/ui/CardTable.svelte -->
<script lang="ts">
  type Props = {
    title: string;
    actions?: import('svelte').Snippet;
    children: import('svelte').Snippet;
  };

  let { title, actions, children }: Props = $props();
</script>

<div class="bg-white rounded-lg shadow-sm">
  <div class="flex items-center justify-between px-6 py-4 border-b border-slate-100">
    <h3 class="text-sm font-semibold text-slate-700">{title}</h3>
    {#if actions}
      <div>{@render actions()}</div>
    {/if}
  </div>
  <div class="overflow-x-auto">
    {@render children()}
  </div>
</div>
```

- [ ] **Step 4: Create CardSettings component**

```svelte
<!-- frontend/src/lib/components/ui/CardSettings.svelte -->
<script lang="ts">
  type Props = {
    id?: string;
    title: string;
    description?: string;
    children: import('svelte').Snippet;
  };

  let { id, title, description, children }: Props = $props();
</script>

<section id={id} class="bg-white rounded-lg shadow-sm p-6 scroll-mt-24">
  <p class="text-xs font-bold uppercase tracking-wider text-slate-500">{title}</p>
  <h3 class="text-lg font-semibold text-slate-800 mt-1">{title}</h3>
  {#if description}
    <p class="text-sm text-slate-500 mt-1">{description}</p>
  {/if}
  <div class="mt-4 space-y-4">
    {@render children()}
  </div>
</section>
```

- [ ] **Step 5: Create EmptyState component**

```svelte
<!-- frontend/src/lib/components/ui/EmptyState.svelte -->
<script lang="ts">
  import type { Component } from 'svelte';

  type Props = {
    icon?: Component;
    title: string;
    description: string;
    items?: string[];
  };

  let { icon, title, description, items = [] }: Props = $props();
</script>

<div class="bg-white rounded-lg shadow-sm p-8 text-center">
  {#if icon}
    <div class="mx-auto w-16 h-16 rounded-full bg-slate-100 flex items-center justify-center mb-4">
      <svelte:component this={icon} class="w-8 h-8 text-slate-400" />
    </div>
  {/if}
  <h3 class="text-lg font-semibold text-slate-700">{title}</h3>
  <p class="text-sm text-slate-500 mt-1">{description}</p>
  {#if items.length > 0}
    <ul class="mt-4 text-sm text-slate-500 text-left max-w-md mx-auto space-y-1">
      {#each items as item}
        <li class="flex items-start gap-2">
          <span class="text-slate-400 mt-0.5">•</span>
          <span>{item}</span>
        </li>
      {/each}
    </ul>
  {/if}
</div>
```

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/components/ui/
git commit -m "feat(ui): add shared UI components (CardStats, CardChart, CardTable, CardSettings, EmptyState)"
```

---

## Task 3: Rebuild Layout Shell (Sidebar + Navbar)

**Files:**
- Create: `frontend/src/lib/components/ui/Sidebar.svelte`
- Create: `frontend/src/lib/components/ui/Navbar.svelte`
- Create: `frontend/src/lib/components/ui/UserDropdown.svelte`
- Modify: `frontend/src/lib/navigation.ts` — Add icon references
- Modify: `frontend/src/routes/+layout.svelte` — Full rebuild

- [ ] **Step 1: Update navigation.ts with icons**

```ts
// frontend/src/lib/navigation.ts
  import {
    LayoutDashboard,
    Users,
    Link,
    History,
    Settings,
    ShieldCheck,
    FileText,
    ClipboardList
  } from 'lucide-svelte';

  export type NavItem = {
    href: string;
    label: string;
    description: string;
    roles?: string[];
    icon: typeof LayoutDashboard;
    section: 'main' | 'management' | 'system';
  };

  const readableRoles = ['owner', 'viewer', 'read_only', 'read-only'];

  export const navItems: NavItem[] = [
    { href: '/', label: 'Dashboard', description: 'Local ocserv status overview', icon: LayoutDashboard, section: 'main' },
    { href: '/users', label: 'Users', description: 'Password and certificate users', icon: Users, section: 'main', roles: ['owner'] },
    { href: '/sessions', label: 'Sessions', description: 'Active session visibility and control', icon: Link, section: 'main', roles: ['owner'] },
    { href: '/history', label: 'History', description: 'Historical session analytics', icon: History, section: 'main', roles: readableRoles },
    { href: '/config', label: 'Config', description: 'Validated configuration editing', icon: Settings, section: 'management', roles: ['owner'] },
    { href: '/certificates', label: 'Certificates', description: 'Certificate visibility and lifecycle', icon: ShieldCheck, section: 'management', roles: ['owner'] },
    { href: '/audit', label: 'Audit', description: 'Administrative action history', icon: FileText, section: 'management', roles: ['owner'] },
    { href: '/settings', label: 'Settings', description: 'Local panel settings', icon: ClipboardList, section: 'system', roles: ['owner'] }
  ];
```

- [ ] **Step 2: Create Sidebar component**

```svelte
<!-- frontend/src/lib/components/ui/Sidebar.svelte -->
<script lang="ts">
  import { page } from '$app/stores';
  import { LogOut, Shield } from 'lucide-svelte';
  import type { NavItem } from '$lib/navigation';

  type Props = {
    navItems: NavItem[];
    username: string;
    roles: string[];
    onLogout: () => void;
    loading: boolean;
    setupRequired: boolean;
    error: string;
  };

  let { navItems, username, roles, onLogout, loading, setupRequired, error }: Props = $props();

  const mainItems = $derived(navItems.filter(i => i.section === 'main'));
  const managementItems = $derived(navItems.filter(i => i.section === 'management'));
  const systemItems = $derived(navItems.filter(i => i.section === 'system'));

  function isActive(href: string): boolean {
    if (href === '/') return $page.url.pathname === '/';
    return $page.url.pathname.startsWith(href);
  }
</script>

<aside class="fixed top-0 left-0 bottom-0 w-64 bg-slate-700 text-slate-300 flex flex-col z-30">
  <!-- Brand -->
  <div class="px-6 py-5">
    <div class="flex items-center gap-2">
      <Shield class="w-6 h-6 text-slate-200" />
      <h1 class="text-lg font-bold text-white tracking-tight">ocserv Panel</h1>
    </div>
    <p class="text-xs text-slate-400 mt-1">Single-node local administration</p>
  </div>

  <!-- Navigation -->
  <nav class="flex-1 overflow-y-auto px-3" aria-label="Primary navigation">
    {#if mainItems.length > 0}
      <p class="px-3 text-xs font-semibold uppercase tracking-wider text-slate-500 mb-2 mt-2">Main</p>
      <ul class="space-y-1">
        {#each mainItems as item}
          <li>
            <a
              href={item.href}
              class="flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors {isActive(item.href) ? 'bg-slate-600/30 text-slate-200' : 'text-slate-400 hover:bg-slate-600/20 hover:text-slate-200'}"
            >
              <svelte:component this={item.icon} class="w-4 h-4" />
              {item.label}
            </a>
          </li>
        {/each}
      </ul>
    {/if}

    {#if managementItems.length > 0}
      <p class="px-3 text-xs font-semibold uppercase tracking-wider text-slate-500 mb-2 mt-4">Management</p>
      <ul class="space-y-1">
        {#each managementItems as item}
          <li>
            <a
              href={item.href}
              class="flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors {isActive(item.href) ? 'bg-slate-600/30 text-slate-200' : 'text-slate-400 hover:bg-slate-600/20 hover:text-slate-200'}"
            >
              <svelte:component this={item.icon} class="w-4 h-4" />
              {item.label}
            </a>
          </li>
        {/each}
      </ul>
    {/if}

    {#if systemItems.length > 0}
      <p class="px-3 text-xs font-semibold uppercase tracking-wider text-slate-500 mb-2 mt-4">System</p>
      <ul class="space-y-1">
        {#each systemItems as item}
          <li>
            <a
              href={item.href}
              class="flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors {isActive(item.href) ? 'bg-slate-600/30 text-slate-200' : 'text-slate-400 hover:bg-slate-600/20 hover:text-slate-200'}"
            >
              <svelte:component this={item.icon} class="w-4 h-4" />
              {item.label}
            </a>
          </li>
        {/each}
      </ul>
    {/if}

    {#if loading}
      <p class="px-3 py-2 text-sm text-slate-400">Checking session…</p>
    {:else if setupRequired}
      <a href="/setup" class="flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium text-slate-400 hover:bg-slate-600/20 hover:text-slate-200 transition-colors">
        Setup owner
      </a>
    {/if}
  </nav>

  <!-- User identity -->
  {#if username}
    <div class="border-t border-slate-600 px-6 py-4">
      <p class="text-xs text-slate-400">Signed in as</p>
      <p class="text-sm font-medium text-slate-200">{username}</p>
      <p class="text-xs text-slate-400">{roles.join(', ')}</p>
      <button
        type="button"
        onclick={onLogout}
        class="mt-2 flex items-center gap-2 text-sm text-slate-400 hover:text-slate-200 transition-colors"
      >
        <LogOut class="w-4 h-4" />
        Logout
      </button>
    </div>
  {/if}

  {#if error}
    <div class="px-6 py-2 text-sm text-red-400">{error}</div>
  {/if}
</aside>
```

- [ ] **Step 3: Create Navbar component**

```svelte
<!-- frontend/src/lib/components/ui/Navbar.svelte -->
<script lang="ts">
  import { User } from 'lucide-svelte';

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

<header class="h-16 bg-white border-b border-slate-200 flex items-center justify-between px-6 sticky top-0 z-20">
  <h2 class="text-lg font-semibold text-slate-700">{title}</h2>

  <div class="relative">
    <button
      type="button"
      onclick={toggleDropdown}
      class="flex items-center gap-2 text-sm text-slate-600 hover:text-slate-800 transition-colors"
    >
      <div class="w-8 h-8 rounded-full bg-slate-200 flex items-center justify-center">
        <User class="w-4 h-4 text-slate-600" />
      </div>
      <span class="hidden md:inline">{username}</span>
    </button>

    {#if showDropdown}
      <div class="absolute right-0 mt-2 w-48 bg-white rounded-lg shadow-lg border border-slate-200 py-1 z-50">
        <button
          type="button"
          onclick={handleLogout}
          class="w-full text-left px-4 py-2 text-sm text-slate-700 hover:bg-slate-50"
        >
          Sign out
        </button>
      </div>
    {/if}
  </div>
</header>
```

- [ ] **Step 4: Rebuild +layout.svelte**

```svelte
<!-- frontend/src/routes/+layout.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import '../app.css';
  import { logout } from '$lib/api';
  import { createAuthState, refreshAuthState } from '$lib/auth';
  import { navItems } from '$lib/navigation';
  import Sidebar from '$lib/components/ui/Sidebar.svelte';
  import Navbar from '$lib/components/ui/Navbar.svelte';

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

<svelte:head>
  <title>ocserv Panel</title>
  <meta name="description" content="Local-first web administration panel for a single ocserv server" />
</svelte:head>

{#if auth.setupRequired || !auth.principal}
  <!-- No sidebar for login/setup pages -->
  <div class="min-h-screen bg-slate-50">
    {@render children()}
  </div>
{:else}
  <div class="min-h-screen bg-slate-50">
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

- [ ] **Step 5: Verify layout renders**

Run: `cd frontend && npm run dev`
Expected: Sidebar and navbar render with slate theme. Navigation works. Auth flow preserved.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/components/ui/Sidebar.svelte frontend/src/lib/components/ui/Navbar.svelte frontend/src/lib/navigation.ts frontend/src/routes/+layout.svelte
git commit -m "feat(ui): rebuild layout shell with slate sidebar and top navbar"
```

---

## Task 4: Rebuild Dashboard Page

**Files:**
- Modify: `frontend/src/routes/+page.svelte` — Full rebuild

- [ ] **Step 1: Rebuild dashboard page with Tailwind + shared components**

The dashboard page must preserve all existing functionality:
- Loading state, setup required, sign-in required, error states
- Dashboard summary (4 stat cards)
- Monitoring section with window selector and refresh
- Metric cards (CPU, Memory, Network in/out)
- Chart grid with TimeSeriesChart → LayerChart
- Analytics summary
- Recent audit table
- Auto-refresh every 15 seconds

Replace the entire `+page.svelte` with a Tailwind-styled version using CardStats, CardChart, CardTable, WindowSelector (rebuilt with Tailwind), and LayerChart.

Key changes:
- `MetricCard` → `CardStats` with Lucide icons (Cpu, MemoryStick, ArrowDown, ArrowUp)
- `TimeSeriesChart` → `CardChart` wrapping LayerChart `Line` component
- `UnavailableMetric` → `CardStats` with `unavailable={true}`
- `AnalyticsCard` → `CardStats`
- Stat cards grid: `grid-cols-1 md:grid-cols-2 xl:grid-cols-4`
- Chart grid: `grid-cols-1 lg:grid-cols-2`
- Audit table: `CardTable`
- Window selector: rebuilt with Tailwind button group styling
- All monitoring logic, API calls, and state management preserved exactly

- [ ] **Step 2: Create WindowSelector with Tailwind styling**

```svelte
<!-- frontend/src/lib/components/ui/WindowSelector.svelte -->
<script lang="ts">
  type Props = {
    value: string;
    windows: string[];
    onchange: (window: string) => void;
  };

  let { value, windows, onchange }: Props = $props();
</script>

<div class="inline-flex rounded-lg bg-slate-100 p-0.5" role="group" aria-label="Monitoring window">
  {#each windows as window}
    <button
      type="button"
      class="px-3 py-1.5 text-sm font-medium rounded-md transition-colors {window === value ? 'bg-white text-slate-700 shadow-sm' : 'text-slate-500 hover:text-slate-700'}"
      aria-pressed={window === value}
      onclick={() => { if (window !== value) onchange(window); }}
    >
      {window}
    </button>
  {/each}
</div>
```

- [ ] **Step 3: Verify dashboard renders with all functionality**

Run: `cd frontend && npm run dev`
Expected: Dashboard shows stat cards with icons, charts in card wrappers, audit table in CardTable. All monitoring, refresh, and window selection works.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/+page.svelte frontend/src/lib/components/ui/WindowSelector.svelte
git commit -m "feat(ui): rebuild dashboard page with Tailwind, CardStats, CardChart, CardTable"
```

---

## Task 5: Rebuild Login and Setup Pages

**Files:**
- Modify: `frontend/src/routes/login/+page.svelte` — Full rebuild
- Modify: `frontend/src/routes/setup/+page.svelte` — Full rebuild

- [ ] **Step 1: Rebuild login page**

Centered card on `bg-slate-50` background. Lucide `Lock` icon at top. Form fields with `@tailwindcss/forms` styling. Slate accent button. Preserve all existing logic (setup detection, transport warning, error handling).

- [ ] **Step 2: Rebuild setup page**

Same centered card pattern. Lucide `Shield` icon. Preserve all existing logic (setup status check, password confirmation, error handling).

- [ ] **Step 3: Verify login and setup pages render correctly**

Run: `cd frontend && npm run dev`
Expected: Login page shows centered card with slate theme. Setup page shows centered card. All form logic works.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/login/+page.svelte frontend/src/routes/setup/+page.svelte
git commit -m "feat(ui): rebuild login and setup pages with Tailwind styling"
```

---

## Task 6: Rebuild Settings Page

**Files:**
- Modify: `frontend/src/routes/settings/+page.svelte` — Full rebuild
- Create: `frontend/src/lib/components/ui/SettingField.svelte` — Rebuilt with Tailwind
- Create: `frontend/src/lib/components/ui/SettingToggle.svelte` — Rebuilt with Tailwind
- Create: `frontend/src/lib/components/ui/SettingsNav.svelte` — Rebuilt with Tailwind
- Create: `frontend/src/lib/components/ui/RestartBanner.svelte` — Rebuilt with Tailwind

- [ ] **Step 1: Create SettingToggle with Tailwind**

```svelte
<!-- frontend/src/lib/components/ui/SettingToggle.svelte -->
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
  <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {checked ? 'translate-x-6' : 'translate-x-1'}" />
</button>
```

- [ ] **Step 2: Create SettingField with Tailwind**

Rebuild the 210-line SettingField with Tailwind classes. Preserve all 7 field types (toggle, readonly, button, select, multi-checkbox, password, text/number). Use Tailwind form styling from `@tailwindcss/forms`.

- [ ] **Step 3: Create SettingsNav with Tailwind**

```svelte
<!-- frontend/src/lib/components/ui/SettingsNav.svelte -->
<script lang="ts">
  import type { SettingsSection } from '$lib/api';

  type Props = {
    sections: SettingsSection[];
    activeSection: string;
  };

  let { sections, activeSection }: Props = $props();
</script>

<nav class="flex gap-2 overflow-x-auto py-2 sticky top-0 z-10 bg-slate-50" aria-label="Settings sections">
  {#each sections as section}
    <a
      href="#{section.id}"
      class="whitespace-nowrap px-4 py-2 rounded-full text-sm font-semibold transition-colors {activeSection === section.id ? 'bg-slate-600 text-white' : 'bg-white border border-slate-200 text-slate-600 hover:bg-slate-100'}"
    >
      {section.label}
    </a>
  {/each}
</nav>
```

- [ ] **Step 4: Create RestartBanner with Tailwind**

```svelte
<!-- frontend/src/lib/components/ui/RestartBanner.svelte -->
<script lang="ts">
  type Props = {
    pendingChanges: string[];
    onRestart: () => void;
    restarting: boolean;
  };

  let { pendingChanges, onRestart, restarting }: Props = $props();
</script>

{#if pendingChanges.length > 0}
  <div class="bg-amber-50 border border-amber-200 rounded-lg p-4">
    <p class="font-semibold text-amber-800">Restart required</p>
    <p class="text-sm text-amber-700 mt-1">
      {restarting
        ? 'Restarting… the page will reload when the service is back.'
        : 'Configuration changes need a service restart to take effect.'}
    </p>
    {#if !restarting}
      <ul class="mt-2 text-sm text-amber-700 list-disc list-inside">
        {#each pendingChanges as change}
          <li>{change}</li>
        {/each}
      </ul>
      <button
        type="button"
        onclick={onRestart}
        class="mt-3 px-4 py-2 bg-red-600 text-white text-sm font-semibold rounded-lg hover:bg-red-700 transition-colors"
      >
        Restart Now
      </button>
    {/if}
  </div>
{/if}
```

- [ ] **Step 5: Rebuild settings page**

Use CardSettings for each section, SettingsNav for section navigation, SettingField for each setting, RestartBanner for restart warning. Preserve all existing logic (intersection observer, save, restart polling, etc.).

- [ ] **Step 6: Verify settings page renders correctly**

Run: `cd frontend && npm run dev`
Expected: Settings page shows section nav, form fields with Tailwind styling, toggle switches, restart banner. All save/restart logic works.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/routes/settings/+page.svelte frontend/src/lib/components/ui/SettingField.svelte frontend/src/lib/components/ui/SettingToggle.svelte frontend/src/lib/components/ui/SettingsNav.svelte frontend/src/lib/components/ui/RestartBanner.svelte
git commit -m "feat(ui): rebuild settings page with Tailwind, CardSettings, SettingField, SettingToggle"
```

---

## Task 7: Rebuild History Page

**Files:**
- Modify: `frontend/src/routes/history/+page.svelte` — Full rebuild

- [ ] **Step 1: Rebuild history page**

Preserve all existing functionality:
- Loading, setup required, sign-in required, error states
- Stat cards (Sessions, Received, Sent, Total traffic)
- Monitoring section with window selector (24h, 7d, 30d)
- Chart grid with LayerChart
- Recent sessions table
- Housekeeping section (retention setting)
- History purge section (owner-only destructive action)
- Auto-refresh every 15 seconds

Use CardStats for stat cards, CardChart for charts, CardTable for sessions table, CardSettings for housekeeping and purge sections. RestartBanner pattern for destructive action warning.

- [ ] **Step 2: Verify history page renders correctly**

Run: `cd frontend && npm run dev`
Expected: History page shows stat cards, charts, session table, housekeeping form, purge form. All functionality preserved.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/history/+page.svelte
git commit -m "feat(ui): rebuild history page with Tailwind, CardStats, CardChart, CardTable"
```

---

## Task 8: Rebuild Placeholder Pages

**Files:**
- Modify: `frontend/src/routes/users/+page.svelte`
- Modify: `frontend/src/routes/sessions/+page.svelte`
- Modify: `frontend/src/routes/config/+page.svelte`
- Modify: `frontend/src/routes/certificates/+page.svelte`
- Modify: `frontend/src/routes/audit/+page.svelte`

- [ ] **Step 1: Rebuild each placeholder page using EmptyState**

Each placeholder page uses the EmptyState component with a relevant Lucide icon:

- Users → `Users` icon
- Sessions → `Link` icon
- Config → `Settings` icon
- Certificates → `ShieldCheck` icon
- Audit → `FileText` icon

Example for users:
```svelte
<script lang="ts">
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import { Users } from 'lucide-svelte';
</script>

<div class="space-y-6">
  <EmptyState
    icon={Users}
    title="User lifecycle management"
    description="Future workflows will manage password and certificate-based ocserv users through authorized backend APIs."
    items={[
      'User creation and deletion will require RBAC authorization.',
      'Password handling will avoid plaintext storage.',
      'Certificate workflows will avoid storing private keys in SQLite.'
    ]}
  />
</div>
```

- [ ] **Step 2: Verify all placeholder pages render**

Run: `cd frontend && npm run dev`
Expected: Each placeholder page shows EmptyState with icon and description.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/users/+page.svelte frontend/src/routes/sessions/+page.svelte frontend/src/routes/config/+page.svelte frontend/src/routes/certificates/+page.svelte frontend/src/routes/audit/+page.svelte
git commit -m "feat(ui): rebuild placeholder pages with EmptyState and Lucide icons"
```

---

## Task 9: Cleanup Old Components and CSS

**Files:**
- Remove: `frontend/src/lib/components/monitoring/MetricCard.svelte`
- Remove: `frontend/src/lib/components/monitoring/TimeSeriesChart.svelte`
- Remove: `frontend/src/lib/components/monitoring/AnalyticsCard.svelte`
- Remove: `frontend/src/lib/components/monitoring/WindowSelector.svelte`
- Remove: `frontend/src/lib/components/monitoring/UnavailableMetric.svelte`
- Remove: `frontend/src/lib/components/settings/SettingsNav.svelte`
- Remove: `frontend/src/lib/components/settings/RestartBanner.svelte`
- Remove: `frontend/src/lib/components/settings/SettingsSection.svelte`
- Remove: `frontend/src/lib/components/settings/SettingField.svelte`
- Remove: `frontend/src/lib/components/settings/SettingToggle.svelte`
- Remove: `frontend/src/lib/PlaceholderPage.svelte`
- Modify: `frontend/src/app.css` — Remove old custom CSS, keep only Tailwind directives

- [ ] **Step 1: Remove old component files**

```bash
rm frontend/src/lib/components/monitoring/MetricCard.svelte
rm frontend/src/lib/components/monitoring/TimeSeriesChart.svelte
rm frontend/src/lib/components/monitoring/AnalyticsCard.svelte
rm frontend/src/lib/components/monitoring/WindowSelector.svelte
rm frontend/src/lib/components/monitoring/UnavailableMetric.svelte
rm frontend/src/lib/components/settings/SettingsNav.svelte
rm frontend/src/lib/components/settings/RestartBanner.svelte
rm frontend/src/lib/components/settings/SettingsSection.svelte
rm frontend/src/lib/components/settings/SettingField.svelte
rm frontend/src/lib/components/settings/SettingToggle.svelte
rm frontend/src/lib/PlaceholderPage.svelte
```

- [ ] **Step 2: Clean up app.css**

Replace the entire `app.css` with just Tailwind directives. The old 616 lines of custom CSS are no longer needed since all styling is now done with Tailwind utility classes.

```css
/* frontend/src/app.css */
@tailwind base;
@tailwind components;
@tailwind utilities;
```

- [ ] **Step 3: Verify no broken imports**

Run: `cd frontend && npm run check`
Expected: No TypeScript/Svelte errors. All imports resolve to new components.

- [ ] **Step 4: Verify full app renders**

Run: `cd frontend && npm run dev`
Expected: All pages render correctly with Tailwind styling. No console errors. All functionality preserved.

- [ ] **Step 5: Commit**

```bash
git add -A frontend/src/lib/components/ frontend/src/lib/PlaceholderPage.svelte frontend/src/app.css
git commit -m "feat(ui): remove old components and custom CSS, replaced by Tailwind utilities"
```

---

## Task 10: Final Verification and Build Test

- [ ] **Step 1: Run Svelte type check**

```bash
cd frontend && npm run check
```

Expected: No errors

- [ ] **Step 2: Run production build**

```bash
cd frontend && npm run build
```

Expected: Build succeeds without errors

- [ ] **Step 3: Manual verification checklist**

Verify each page renders correctly:
- [ ] Login page: centered card, form fields, transport warning
- [ ] Setup page: centered card, password confirmation
- [ ] Dashboard: stat cards with icons, monitoring charts, audit table
- [ ] History: stat cards, charts, session table, housekeeping, purge
- [ ] Settings: section nav, form fields, toggles, restart banner
- [ ] Users, Sessions, Config, Certificates, Audit: EmptyState with icons
- [ ] Sidebar: navigation with icons, section headings, active state, user identity, logout
- [ ] Navbar: page title, user dropdown
- [ ] Mobile responsive: sidebar collapses, grids stack

- [ ] **Step 4: Commit final state**

```bash
git add -A
git commit -m "feat(ui): complete UI overhaul with Tailwind CSS, Lucide icons, and LayerChart"
```