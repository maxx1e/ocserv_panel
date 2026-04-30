<script lang="ts">
  import { onMount } from 'svelte';
  import '../app.css';
  import { logout } from '$lib/api';
  import { createAuthState, refreshAuthState } from '$lib/auth';
  import { navItems } from '$lib/navigation';

  let { children } = $props();
  let auth = $state(createAuthState());
  let logoutError = $state('');

  const visibleNavItems = $derived(
    auth.principal
      ? navItems.filter((item) => !item.roles || item.roles.some((role) => auth.principal?.roles?.includes(role)))
      : []
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
  <meta
    name="description"
      content="Local-first web administration panel for a single ocserv server"
  />
</svelte:head>

<div class="shell">
  <aside class="sidebar" aria-label="Primary navigation">
    <h1 class="brand">ocserv Panel</h1>
    <p class="sidebar-note">Single-node local administration via backend APIs only.</p>
    <nav class="nav">
      {#each visibleNavItems as item}
        <a href={item.href} title={item.description}>{item.label}</a>
      {/each}
      {#if auth.loading}
        <span class="nav-status">Checking session…</span>
      {:else if auth.setupRequired}
        <a href="/setup">Setup owner</a>
      {:else if auth.principal}
        <button type="button" class="nav-button" onclick={handleLogout}>Logout</button>
      {:else}
        <a href="/login">Login</a>
      {/if}
    </nav>
    {#if auth.principal}
      <div class="identity">
        <span>Signed in as</span>
        <strong>{auth.principal.username}</strong>
        <small>{auth.principal.roles?.join(', ') || 'role pending'}</small>
      </div>
    {/if}
    {#if auth.error || logoutError}
      <p class="sidebar-error">{auth.error || logoutError}</p>
    {/if}
  </aside>

  <main class="content">
    {@render children()}
  </main>
</div>
