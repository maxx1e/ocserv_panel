<script lang="ts">
  import { onMount } from 'svelte';
  import { Users, Shield } from 'lucide-svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import {
    getAdminUsers,
    getVpnUsers,
    getMe,
    getSetupStatus,
    principalFromAuth,
    hasOwnerRole,
    ApiError,
    type AdminUser,
    type VpnUser,
    type Principal
  } from '$lib/api';
  import AdminUsersTab from './AdminUsersTab.svelte';
  import VpnUsersTab from './VpnUsersTab.svelte';

  // ── page state ────────────────────────────────────────────────────────────

  let loading = $state(true);
  let error = $state('');
  let forbidden = $state(false);
  let setupRequired = $state(false);
  let principal = $state<Principal | null>(null);

  let activeTab = $state<'admin' | 'vpn'>('admin');
  let adminUsers = $state<AdminUser[]>([]);
  let vpnUsers = $state<VpnUser[]>([]);
  let adminLoading = $state(false);
  let vpnLoading = $state(false);
  let adminError = $state('');
  let vpnError = $state('');

  const isOwner = $derived(hasOwnerRole(principal));

  // ── lifecycle ─────────────────────────────────────────────────────────────

  onMount(() => {
    void initialize();
  });

  async function initialize() {
    loading = true;
    error = '';
    forbidden = false;
    setupRequired = false;

    try {
      const setup = await getSetupStatus();
      setupRequired = setup.required;
      if (setup.required) return;

      principal = principalFromAuth(await getMe());
      await Promise.all([loadAdminUsers(), loadVpnUsers()]);
    } catch (caught) {
      if (caught instanceof ApiError) {
        if (caught.status === 403) {
          forbidden = true;
          return;
        }
      }
      error = caught instanceof Error ? caught.message : 'Unable to load users.';
    } finally {
      loading = false;
    }
  }

  async function loadAdminUsers() {
    adminLoading = true;
    adminError = '';
    try {
      const response = await getAdminUsers();
      adminUsers = response.users;
    } catch (caught) {
      adminError = caught instanceof Error ? caught.message : 'Failed to load admin users.';
    } finally {
      adminLoading = false;
    }
  }

  async function loadVpnUsers() {
    vpnLoading = true;
    vpnError = '';
    try {
      vpnUsers = await getVpnUsers();
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 503) {
        vpnError = 'VPN user management is unavailable. Is ocserv configured?';
      } else {
        vpnError = caught instanceof Error ? caught.message : 'Failed to load VPN users.';
      }
    } finally {
      vpnLoading = false;
    }
  }
</script>

<!-- Loading -->
{#if loading}
  <div class="flex items-center justify-center py-16">
    <div class="text-center">
      <div class="w-8 h-8 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin mx-auto mb-3"></div>
      <p class="text-slate-500 text-sm">Loading users...</p>
    </div>
  </div>

<!-- Setup required -->
{:else if setupRequired}
  <div class="bg-amber-50 border border-amber-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-amber-800">Initial setup required</h3>
    <p class="text-amber-700 text-sm mt-1">Create the first owner before managing users.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-amber-600 text-white text-sm font-medium rounded-md hover:bg-amber-700 transition-colors" href="/setup">
      Set up owner
    </a>
  </div>

<!-- Forbidden -->
{:else if forbidden}
  <EmptyState
    icon={Shield}
    title="Access denied"
    description="Your role cannot manage users."
  />

<!-- Not authenticated -->
{:else if !principal}
  <div class="bg-slate-50 border border-slate-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-slate-700">Sign in required</h3>
    <p class="text-slate-500 text-sm mt-1">User management requires authentication.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-slate-700 text-white text-sm font-medium rounded-md hover:bg-slate-800 transition-colors" href="/login">
      Go to login
    </a>
    {#if error}
      <p class="mt-3 text-sm text-red-600">{error}</p>
    {/if}
  </div>

<!-- Main view -->
{:else}
  <div class="space-y-6">
    <!-- Page header -->
    <div>
      <h1 class="text-2xl font-bold text-slate-800 dark:text-slate-100">Users</h1>
      <p class="text-sm text-slate-500 dark:text-slate-400 mt-1">Manage user accounts and permissions.</p>
    </div>

    <!-- Tab buttons -->
    <div class="flex border-b border-slate-200 dark:border-slate-700">
      <button
        type="button"
        onclick={() => (activeTab = 'admin')}
        class="px-4 py-2.5 text-sm font-medium border-b-2 transition-colors {activeTab === 'admin'
          ? 'border-blue-600 text-blue-600 dark:text-blue-400 dark:border-blue-400'
          : 'border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:border-slate-300'}"
      >
        <span class="inline-flex items-center gap-2">
          <Users class="w-4 h-4" />
          Web UI Users
        </span>
      </button>
      <button
        type="button"
        onclick={() => (activeTab = 'vpn')}
        class="px-4 py-2.5 text-sm font-medium border-b-2 transition-colors {activeTab === 'vpn'
          ? 'border-blue-600 text-blue-600 dark:text-blue-400 dark:border-blue-400'
          : 'border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:border-slate-300'}"
      >
        <span class="inline-flex items-center gap-2">
          <Shield class="w-4 h-4" />
          VPN Users
        </span>
      </button>
    </div>

    <!-- Tab content -->
    {#if activeTab === 'admin'}
      {#if adminLoading}
        <div class="flex items-center justify-center py-10">
          <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
        </div>
      {:else if adminError}
        <div class="px-4 py-3 rounded-lg text-sm bg-red-50 dark:bg-red-900/20 text-red-800 dark:text-red-200" role="alert">
          {adminError}
        </div>
      {:else}
        <AdminUsersTab users={adminUsers} {isOwner} onRefresh={loadAdminUsers} />
      {/if}
    {:else}
      {#if vpnLoading}
        <div class="flex items-center justify-center py-10">
          <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
        </div>
      {:else if vpnError}
        <div class="px-4 py-3 rounded-lg text-sm bg-red-50 dark:bg-red-900/20 text-red-800 dark:text-red-200" role="alert">
          {vpnError}
        </div>
      {:else}
        <VpnUsersTab users={vpnUsers} {isOwner} onRefresh={loadVpnUsers} />
      {/if}
    {/if}
  </div>
{/if}
