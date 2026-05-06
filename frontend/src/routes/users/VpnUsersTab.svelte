<script lang="ts">
  import { Search, Plus, Lock, Unlock, KeyRound, Trash2, ArrowUpDown } from 'lucide-svelte';
  import { paginate } from '$lib/utils/pagination';
  import {
    lockVpnUser,
    unlockVpnUser,
    resetVpnPassword,
    deleteVpnUser,
    type VpnUser
  } from '$lib/api';
  import VpnUserFormDialog from './VpnUserFormDialog.svelte';
  import DeleteConfirmDialog from './DeleteConfirmDialog.svelte';
  import ResetPasswordDialog from './ResetPasswordDialog.svelte';

  type Props = {
    users: VpnUser[];
    isOwner: boolean;
    onRefresh: () => void;
  };

  let { users, isOwner, onRefresh }: Props = $props();

  // ── local state ───────────────────────────────────────────────────────────

  let search = $state('');
  let statusFilter = $state<'all' | 'active' | 'locked'>('all');
  let sortColumn = $state<'username' | 'group' | 'status'>('username');
  let sortDirection = $state<'asc' | 'desc'>('asc');
  let page = $state(1);
  let pageSize = $state(10);

  // ── dialog state ──────────────────────────────────────────────────────────

  let createOpen = $state(false);
  let deleteOpen = $state(false);
  let deleteTarget = $state<VpnUser | null>(null);
  let resetPwdOpen = $state(false);
  let resetPwdTarget = $state<VpnUser | null>(null);

  // ── notifications ─────────────────────────────────────────────────────────

  let successMsg = $state('');
  let errorMsg = $state('');

  // ── derived ───────────────────────────────────────────────────────────────

  const filteredUsers = $derived((() => {
    let result = users;

    if (search.trim()) {
      const q = search.trim().toLowerCase();
      result = result.filter(u => u.username.toLowerCase().includes(q));
    }

    if (statusFilter === 'active') {
      result = result.filter(u => u.active);
    } else if (statusFilter === 'locked') {
      result = result.filter(u => !u.active);
    }

    return result;
  })());

  const sortedUsers = $derived((() => {
    const copy = [...filteredUsers];
    copy.sort((a, b) => {
      let cmp = 0;
      switch (sortColumn) {
        case 'username': cmp = a.username.localeCompare(b.username); break;
        case 'group': cmp = (a.group || '').localeCompare(b.group || ''); break;
        case 'status': cmp = Number(b.active) - Number(a.active); break;
      }
      return sortDirection === 'asc' ? cmp : -cmp;
    });
    return copy;
  })());

  const paged = $derived(paginate(sortedUsers, page, pageSize));

  // Reset page when filters change
  $effect(() => {
    filteredUsers;
    page = 1;
  });

  // ── helpers ───────────────────────────────────────────────────────────────

  function toggleSort(col: typeof sortColumn) {
    if (sortColumn === col) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortColumn = col;
      sortDirection = 'asc';
    }
  }

  function openDelete(user: VpnUser) {
    deleteTarget = user;
    deleteOpen = true;
  }

  function openResetPwd(user: VpnUser) {
    resetPwdTarget = user;
    resetPwdOpen = true;
  }

  async function handleToggleLock(user: VpnUser) {
    try {
      if (user.active) {
        await lockVpnUser(user.username);
        successMsg = `VPN user "${user.username}" locked.`;
      } else {
        await unlockVpnUser(user.username);
        successMsg = `VPN user "${user.username}" unlocked.`;
      }
      onRefresh();
    } catch (caught) {
      errorMsg = caught instanceof Error ? caught.message : 'Operation failed.';
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    await deleteVpnUser(deleteTarget.username);
    successMsg = `VPN user "${deleteTarget.username}" deleted.`;
    deleteTarget = null;
    onRefresh();
  }

  async function handleResetPassword(newPassword: string) {
    if (!resetPwdTarget) return;
    await resetVpnPassword(resetPwdTarget.username, newPassword);
    successMsg = `Password reset for VPN user "${resetPwdTarget.username}".`;
    resetPwdTarget = null;
    onRefresh();
  }

  function handleCreateSuccess() {
    successMsg = 'VPN user created.';
    onRefresh();
  }
</script>

<!-- Notifications -->
{#if successMsg}
  <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-green-50 dark:bg-green-900/20 text-green-800 dark:text-green-200 mb-4" role="status">
    <span>{successMsg}</span>
    <button type="button" onclick={() => (successMsg = '')} class="ml-4 text-green-600 hover:text-green-800" aria-label="Dismiss">&times;</button>
  </div>
{/if}
{#if errorMsg}
  <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-red-50 dark:bg-red-900/20 text-red-800 dark:text-red-200 mb-4" role="alert">
    <span>{errorMsg}</span>
    <button type="button" onclick={() => (errorMsg = '')} class="ml-4 text-red-600 hover:text-red-800" aria-label="Dismiss">&times;</button>
  </div>
{/if}

<!-- Toolbar -->
<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4">
  <div class="flex items-center gap-3 flex-wrap">
    <!-- Search -->
    <div class="relative">
      <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
      <input
        type="text"
        bind:value={search}
        placeholder="Search VPN users..."
        class="pl-9 pr-3 py-2 w-56 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-sm text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
      />
    </div>

    <!-- Status filter -->
    <select
      bind:value={statusFilter}
      class="px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-sm text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500"
    >
      <option value="all">All Status</option>
      <option value="active">Active</option>
      <option value="locked">Locked</option>
    </select>

    <!-- Count badge -->
    <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-slate-100 dark:bg-slate-700 text-slate-700 dark:text-slate-300">
      {filteredUsers.length} user{filteredUsers.length !== 1 ? 's' : ''}
    </span>
  </div>

  {#if isOwner}
    <button
      type="button"
      onclick={() => (createOpen = true)}
      class="inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium rounded-lg bg-green-600 text-white hover:bg-green-700 transition-colors"
    >
      <Plus class="w-4 h-4" />
      Add VPN User
    </button>
  {/if}
</div>

<!-- Table -->
<div class="bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
  {#if filteredUsers.length === 0}
    <div class="px-6 py-10 text-center">
      <p class="text-sm text-slate-500 dark:text-slate-400">No VPN users match the current filters.</p>
    </div>
  {:else}
    <div class="overflow-x-auto">
      <table class="w-full">
        <thead>
          <tr class="bg-slate-50 dark:bg-slate-800">
            <th
              class="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-700 select-none"
              onclick={() => toggleSort('username')}
            >
              <span class="inline-flex items-center gap-1">Username <ArrowUpDown class="w-3 h-3" /></span>
            </th>
            <th
              class="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-700 select-none"
              onclick={() => toggleSort('group')}
            >
              <span class="inline-flex items-center gap-1">Group <ArrowUpDown class="w-3 h-3" /></span>
            </th>
            <th
              class="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-700 select-none"
              onclick={() => toggleSort('status')}
            >
              <span class="inline-flex items-center gap-1">Status <ArrowUpDown class="w-3 h-3" /></span>
            </th>
            {#if isOwner}
              <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                Actions
              </th>
            {/if}
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200 dark:divide-slate-700">
          {#each paged.items as user (user.username)}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors">
              <td class="px-4 py-3 text-sm font-medium text-slate-900 dark:text-slate-100">
                {user.username}
              </td>
              <td class="px-4 py-3 text-sm text-slate-700 dark:text-slate-300">
                {user.group || '--'}
              </td>
              <td class="px-4 py-3 text-sm">
                {#if user.active}
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300">
                    Active
                  </span>
                {:else}
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300">
                    Locked
                  </span>
                {/if}
              </td>
              {#if isOwner}
                <td class="px-4 py-3">
                  <div class="flex items-center gap-1">
                    <button
                      type="button"
                      onclick={() => handleToggleLock(user)}
                      title={user.active ? 'Lock user' : 'Unlock user'}
                      class="p-1.5 rounded-md text-slate-400 hover:text-amber-600 hover:bg-amber-50 dark:hover:bg-amber-900/20 transition-colors"
                    >
                      {#if user.active}
                        <Lock class="w-4 h-4" />
                      {:else}
                        <Unlock class="w-4 h-4" />
                      {/if}
                    </button>
                    <button
                      type="button"
                      onclick={() => openResetPwd(user)}
                      title="Reset password"
                      class="p-1.5 rounded-md text-slate-400 hover:text-slate-600 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors"
                    >
                      <KeyRound class="w-4 h-4" />
                    </button>
                    <button
                      type="button"
                      onclick={() => openDelete(user)}
                      title="Delete user"
                      class="p-1.5 rounded-md text-slate-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
                    >
                      <Trash2 class="w-4 h-4" />
                    </button>
                  </div>
                </td>
              {/if}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <!-- Pagination footer -->
    <div class="flex items-center justify-between px-4 py-3 border-t border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/50">
      <div class="flex items-center gap-2">
        <span class="text-sm text-slate-500 dark:text-slate-400">Rows per page:</span>
        <select
          bind:value={pageSize}
          class="px-2 py-1 text-sm border border-slate-300 dark:border-slate-600 rounded bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-200"
        >
          <option value={10}>10</option>
          <option value={25}>25</option>
          <option value={50}>50</option>
        </select>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-sm text-slate-500 dark:text-slate-400">
          Page {paged.page} of {paged.totalPages}
        </span>
        <button
          type="button"
          disabled={paged.page <= 1}
          onclick={() => (page = paged.page - 1)}
          class="px-2.5 py-1 text-sm font-medium rounded border border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-300 bg-white dark:bg-slate-800 hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          Prev
        </button>
        <button
          type="button"
          disabled={paged.page >= paged.totalPages}
          onclick={() => (page = paged.page + 1)}
          class="px-2.5 py-1 text-sm font-medium rounded border border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-300 bg-white dark:bg-slate-800 hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          Next
        </button>
      </div>
    </div>
  {/if}
</div>

<!-- Dialogs -->
<VpnUserFormDialog bind:open={createOpen} onSuccess={handleCreateSuccess} />
<DeleteConfirmDialog bind:open={deleteOpen} itemName={deleteTarget?.username ?? ''} onConfirm={handleDelete} />
<ResetPasswordDialog bind:open={resetPwdOpen} username={resetPwdTarget?.username ?? ''} onConfirm={handleResetPassword} />
