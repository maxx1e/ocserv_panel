<script lang="ts">
  import { onMount } from 'svelte';
  import { ModeWatcher, setMode } from 'mode-watcher';
  import '../app.css';
  import { logout, getSettingsSections } from '$lib/api';
  import { createAuthState, refreshAuthState } from '$lib/auth';
  import { navItems } from '$lib/navigation';
  import Sidebar from '$lib/components/ui/Sidebar.svelte';
  import Navbar from '$lib/components/ui/Navbar.svelte';
  import { page } from '$app/stores';

  let { children } = $props();
  let auth = $state(createAuthState());
  let logoutError = $state('');
  let mobileMenuOpen = $state(false);

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

  onMount(async () => {
    await refreshAuthState(auth);

    if (auth.principal) {
      try {
        const settings = await getSettingsSections();
        const themeSetting = settings.sections
          .flatMap(s => s.settings)
          .find(s => s.key === 'general.theme');
        if (themeSetting && (themeSetting.value === 'light' || themeSetting.value === 'dark' || themeSetting.value === 'system')) {
          setMode(themeSetting.value);
        }
      } catch {
        // theme load failure is non-critical
      }
    }
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

{#if auth.loading}
  <div class="min-h-screen bg-background flex items-center justify-center">
    <div class="text-muted-foreground text-sm">Loading...</div>
  </div>
{:else if auth.setupRequired || !auth.principal}
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
      mobileOpen={mobileMenuOpen}
      onCloseMobile={() => mobileMenuOpen = false}
    />
    <div class="md:ml-64">
      <Navbar
        title={pageTitle}
        username={auth.principal?.username ?? ''}
        onLogout={handleLogout}
        onToggleSidebar={() => mobileMenuOpen = !mobileMenuOpen}
      />
      <main class="p-6">
        {@render children()}
      </main>
    </div>
  </div>
{/if}
