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
    mobileOpen?: boolean;
    onCloseMobile?: () => void;
  };

  let { navItems, username, roles, onLogout, loading, setupRequired, error, mobileOpen = false, onCloseMobile }: Props = $props();

  const mainItems = $derived(navItems.filter(i => i.section === 'main'));
  const managementItems = $derived(navItems.filter(i => i.section === 'management'));
  const systemItems = $derived(navItems.filter(i => i.section === 'system'));

  function isActive(href: string): boolean {
    if (href === '/') return $page.url.pathname === '/';
    return $page.url.pathname.startsWith(href);
  }
</script>

<!-- Backdrop overlay for mobile -->
{#if mobileOpen}
  <button
    type="button"
    class="fixed inset-0 bg-black/50 z-20 md:hidden cursor-default"
    onclick={onCloseMobile}
    aria-label="Close navigation menu"
  ></button>
{/if}

<aside class="fixed top-0 left-0 bottom-0 w-64 bg-slate-700 text-slate-300 flex-col z-30 {mobileOpen ? 'flex' : 'hidden md:flex'}">
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
          {@const Icon = item.icon}
          <li>
            <a
              href={item.href}
              onclick={onCloseMobile}
              class="flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors {isActive(item.href) ? 'bg-slate-600/30 text-slate-200' : 'text-slate-400 hover:bg-slate-600/20 hover:text-slate-200'}"
            >
              <Icon class="w-4 h-4" />
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
          {@const Icon = item.icon}
          <li>
            <a
              href={item.href}
              onclick={onCloseMobile}
              class="flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors {isActive(item.href) ? 'bg-slate-600/30 text-slate-200' : 'text-slate-400 hover:bg-slate-600/20 hover:text-slate-200'}"
            >
              <Icon class="w-4 h-4" />
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
          {@const Icon = item.icon}
          <li>
            <a
              href={item.href}
              onclick={onCloseMobile}
              class="flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors {isActive(item.href) ? 'bg-slate-600/30 text-slate-200' : 'text-slate-400 hover:bg-slate-600/20 hover:text-slate-200'}"
            >
              <Icon class="w-4 h-4" />
              {item.label}
            </a>
          </li>
        {/each}
      </ul>
    {/if}

    {#if loading}
      <p class="px-3 py-2 text-sm text-slate-400">Checking session...</p>
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
