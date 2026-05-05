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
