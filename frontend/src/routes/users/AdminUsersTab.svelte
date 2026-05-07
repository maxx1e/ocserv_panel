<script lang="ts">
  import { Search, Plus, Download, Pencil, Ban, CheckCircle2, KeyRound, Trash2, ArrowUpDown } from 'lucide-svelte';
  import { paginate } from '$lib/utils/pagination';
  import { formatRelativeTime } from '$lib/utils/format';
  import {
    disableAdminUser,
    enableAdminUser,
    resetAdminPassword,
    deleteAdminUser,
    bulkDeleteAdminUsers,
    bulkDisableAdminUsers,
    bulkEnableAdminUsers,
    type AdminUser,
    type BulkResult
  } from '$lib/api';
  import AdminUserFormDialog from './AdminUserFormDialog.svelte';
  import DeleteConfirmDialog from './DeleteConfirmDialog.svelte';
  import ResetPasswordDialog from './ResetPasswordDialog.svelte';
  import BulkActionToolbar from './BulkActionToolbar.svelte';
  import BulkConfirmDialog from './BulkConfirmDialog.svelte';

  type Props = {
    users: AdminUser[];
    isOwner: boolean;
    onRefresh: () => void;
  };

  let { users, isOwner, onRefresh }: Props = $props();

  // ── local state ───────────────────────────────────────────────────────────

  let search = $state('');
  let roleFilter = $state<'all' | 'owner' | 'viewer'>('all');
  let statusFilter = $state<'all' | 'active' | 'disabled'>('all');
  let sortColumn = $state<'username' | 'displayName' | 'email' | 'role' | 'status' | 'createdAt'>('username');
  let sortDirection = $state<'asc' | 'desc'>('asc');
  let page = $state(1);
  let pageSize = $state(10);

  // ── dialog state ──────────────────────────────────────────────────────────

  let formOpen = $state(false);
  let formMode = $state<'create' | 'edit'>('create');
  let formUser = $state<AdminUser | undefined>(undefined);
  let deleteOpen = $state(false);
  let deleteTarget = $state<AdminUser | null>(null);
  let resetPwdOpen = $state(false);
  let resetPwdTarget = $state<AdminUser | null>(null);

  // ── notifications ─────────────────────────────────────────────────────────

  let successMsg = $state('');
  let errorMsg = $state('');

  // ── bulk selection ────────────────────────────────────────────────────────

  let selectedIds = $state<Set<number>>(new Set());
  let bulkAction = $state<'delete' | 'disable' | 'enable' | null>(null);
  let bulkLoading = $state(false);
  let bulkResultMsg = $state('');
  let bulkErrorMsg = $state('');

  const selectedCount = $derived(selectedIds.size);

  // ── derived ───────────────────────────────────────────────────────────────

  const filteredUsers = $derived((() => {
    let result = users;

    if (search.trim()) {
      const q = search.trim().toLowerCase();
      result = result.filter(u =>
        u.username.toLowerCase().includes(q) ||
        (u.displayName ?? '').toLowerCase().includes(q) ||
        (u.email ?? '').toLowerCase().includes(q)
      );
    }

    if (roleFilter !== 'all') {
      result = result.filter(u => u.roles?.includes(roleFilter));
    }

    if (statusFilter === 'active') {
      result = result.filter(u => !u.disabled);
    } else if (statusFilter === 'disabled') {
      result = result.filter(u => u.disabled);
    }

    return result;
  })());

  const sortedUsers = $derived((() => {
    const copy = [...filteredUsers];
    copy.sort((a, b) => {
      let cmp = 0;
      switch (sortColumn) {
        case 'username': cmp = a.username.localeCompare(b.username); break;
        case 'displayName': cmp = (a.displayName || '').localeCompare(b.displayName || ''); break;
        case 'email': cmp = (a.email || '').localeCompare(b.email || ''); break;
        case 'role': cmp = (a.roles?.[0] || '').localeCompare(b.roles?.[0] || ''); break;
        case 'status': cmp = Number(a.disabled) - Number(b.disabled); break;
        case 'createdAt': cmp = (a.createdAt || '').localeCompare(b.createdAt || ''); break;
      }
      return sortDirection === 'asc' ? cmp : -cmp;
    });
    return copy;
  })());

  const paged = $derived(paginate(sortedUsers, page, pageSize));

  // Reset page when filters change
  $effect(() => {
    // Reference the derived to track changes
    filteredUsers;
    page = 1;
    selectedIds = new Set();
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

  function openCreate() {
    formMode = 'create';
    formUser = undefined;
    formOpen = true;
  }

  function openEdit(user: AdminUser) {
    formMode = 'edit';
    formUser = user;
    formOpen = true;
  }

  function openDelete(user: AdminUser) {
    deleteTarget = user;
    deleteOpen = true;
  }

  function openResetPwd(user: AdminUser) {
    resetPwdTarget = user;
    resetPwdOpen = true;
  }

  async function handleToggleDisable(user: AdminUser) {
    try {
      if (user.disabled) {
        await enableAdminUser(user.id);
        successMsg = `User "${user.username}" enabled.`;
      } else {
        await disableAdminUser(user.id);
        successMsg = `User "${user.username}" disabled.`;
      }
      onRefresh();
    } catch (caught) {
      errorMsg = caught instanceof Error ? caught.message : 'Operation failed.';
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    await deleteAdminUser(deleteTarget.id);
    successMsg = `User "${deleteTarget.username}" deleted.`;
    deleteTarget = null;
    onRefresh();
  }

  async function handleResetPassword(newPassword: string) {
    if (!resetPwdTarget) return;
    await resetAdminPassword(resetPwdTarget.id, newPassword);
    successMsg = `Password reset for "${resetPwdTarget.username}".`;
    resetPwdTarget = null;
    onRefresh();
  }

  function handleFormSuccess() {
    successMsg = formMode === 'create' ? 'User created.' : 'User updated.';
    onRefresh();
  }

  function exportCSV() {
    const header = 'Username,Display Name,Email,Role,Status,Created\n';
    const rows = filteredUsers.map(u =>
      `"${u.username}","${u.displayName || ''}","${u.email || ''}","${u.roles?.[0] || ''}","${u.disabled ? 'Disabled' : 'Active'}","${u.createdAt || ''}"`
    ).join('\n');
    downloadFile(header + rows, 'admin-users.csv', 'text/csv');
  }

  function exportJSON() {
    const data = JSON.stringify(filteredUsers, null, 2);
    downloadFile(data, 'admin-users.json', 'application/json');
  }

  function downloadFile(content: string, filename: string, mimeType: string) {
    const blob = new Blob([content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  }

  // ── bulk helpers ──────────────────────────────────────────────────────────

  function toggleSelect(id: number) {
    const next = new Set(selectedIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    selectedIds = next;
  }

  function toggleSelectAll() {
    if (selectedIds.size === paged.items.length && paged.items.length > 0) {
      selectedIds = new Set();
    } else {
      selectedIds = new Set(paged.items.map(u => u.id));
    }
  }

  function clearSelection() {
    selectedIds = new Set();
  }

  async function executeBulkAction() {
    if (!bulkAction) return;
    bulkLoading = true;
    bulkResultMsg = '';
    bulkErrorMsg = '';

    try {
      const ids = [...selectedIds];
      let result: BulkResult;

      switch (bulkAction) {
        case 'delete':
          result = await bulkDeleteAdminUsers(ids);
          break;
        case 'disable':
          result = await bulkDisableAdminUsers(ids);
          break;
        case 'enable':
          result = await bulkEnableAdminUsers(ids);
          break;
      }

      if (result.failed > 0) {
        bulkErrorMsg = `${result.succeeded} succeeded, ${result.failed} failed: ${result.errors.map(e => e.error).join(', ')}`;
      } else {
        bulkResultMsg = `Successfully ${bulkAction === 'delete' ? 'deleted' : bulkAction === 'disable' ? 'disabled' : 'enabled'} ${result.succeeded} user${result.succeeded !== 1 ? 's' : ''}`;
      }

      clearSelection();
      onRefresh();
    } catch (err: any) {
      bulkErrorMsg = err.message || 'Bulk operation failed';
    } finally {
      bulkLoading = false;
      bulkAction = null;
    }
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

{#if bulkResultMsg}
  <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-green-50 dark:bg-green-900/20 text-green-800 dark:text-green-200 mb-4" role="status">
    <span>{bulkResultMsg}</span>
    <button type="button" onclick={() => (bulkResultMsg = '')} class="ml-4 text-green-600 hover:text-green-800" aria-label="Dismiss">&times;</button>
  </div>
{/if}
{#if bulkErrorMsg}
  <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-red-50 dark:bg-red-900/20 text-red-800 dark:text-red-200 mb-4" role="alert">
    <span>{bulkErrorMsg}</span>
    <button type="button" onclick={() => (bulkErrorMsg = '')} class="ml-4 text-red-600 hover:text-red-800" aria-label="Dismiss">&times;</button>
  </div>
{/if}

{#if isOwner}
  <BulkActionToolbar
    count={selectedCount}
    actions={[
      { label: 'Delete', variant: 'destructive', action: () => { bulkAction = 'delete'; } },
      { label: 'Disable', variant: 'default', action: () => { bulkAction = 'disable'; } },
      { label: 'Enable', variant: 'default', action: () => { bulkAction = 'enable'; } }
    ]}
    onClear={clearSelection}
  />
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
        placeholder="Search users..."
        class="pl-9 pr-3 py-2 w-56 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-sm text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
      />
    </div>

    <!-- Role filter -->
    <select
      bind:value={roleFilter}
      class="px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-sm text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500"
    >
      <option value="all">All Roles</option>
      <option value="owner">Owner</option>
      <option value="viewer">Viewer</option>
    </select>

    <!-- Status filter -->
    <select
      bind:value={statusFilter}
      class="px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-sm text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500"
    >
      <option value="all">All Status</option>
      <option value="active">Active</option>
      <option value="disabled">Disabled</option>
    </select>

    <!-- Count badge -->
    <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-slate-100 dark:bg-slate-700 text-slate-700 dark:text-slate-300">
      {filteredUsers.length} user{filteredUsers.length !== 1 ? 's' : ''}
    </span>
  </div>

  {#if isOwner}
    <div class="flex items-center gap-2">
      <!-- Export -->
      <div class="relative group">
        <button
          type="button"
          class="inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium rounded-lg border border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-300 bg-white dark:bg-slate-800 hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors"
        >
          <Download class="w-4 h-4" />
          Export
        </button>
        <div class="absolute right-0 mt-1 w-32 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-600 rounded-lg shadow-lg hidden group-focus-within:block z-10">
          <button type="button" onclick={exportCSV} class="w-full text-left px-3 py-2 text-sm text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-700 rounded-t-lg">CSV</button>
          <button type="button" onclick={exportJSON} class="w-full text-left px-3 py-2 text-sm text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-700 rounded-b-lg">JSON</button>
        </div>
      </div>

      <!-- Add User -->
      <button
        type="button"
        onclick={openCreate}
        class="inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition-colors"
      >
        <Plus class="w-4 h-4" />
        Add User
      </button>
    </div>
  {/if}
</div>

<!-- Table -->
<div class="bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
  {#if filteredUsers.length === 0}
    <div class="px-6 py-10 text-center">
      <p class="text-sm text-slate-500 dark:text-slate-400">No users match the current filters.</p>
    </div>
  {:else}
    <div class="overflow-x-auto">
      <table class="w-full">
        <thead>
          <tr class="bg-slate-50 dark:bg-slate-800">
            {#if isOwner}
              <th class="w-10 px-3 py-3">
                <input
                  type="checkbox"
                  class="h-4 w-4 rounded border-slate-300 text-blue-600"
                  checked={selectedIds.size === paged.items.length && paged.items.length > 0}
                  onchange={toggleSelectAll}
                />
              </th>
            {/if}
            <th
              class="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-700 select-none"
              onclick={() => toggleSort('username')}
            >
              <span class="inline-flex items-center gap-1">Username <ArrowUpDown class="w-3 h-3" /></span>
            </th>
            <th
              class="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-700 select-none"
              onclick={() => toggleSort('displayName')}
            >
              <span class="inline-flex items-center gap-1">Display Name <ArrowUpDown class="w-3 h-3" /></span>
            </th>
            <th
              class="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-700 select-none"
              onclick={() => toggleSort('email')}
            >
              <span class="inline-flex items-center gap-1">Email <ArrowUpDown class="w-3 h-3" /></span>
            </th>
            <th
              class="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-700 select-none"
              onclick={() => toggleSort('role')}
            >
              <span class="inline-flex items-center gap-1">Role <ArrowUpDown class="w-3 h-3" /></span>
            </th>
            <th
              class="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-700 select-none"
              onclick={() => toggleSort('status')}
            >
              <span class="inline-flex items-center gap-1">Status <ArrowUpDown class="w-3 h-3" /></span>
            </th>
            <th
              class="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-700 select-none"
              onclick={() => toggleSort('createdAt')}
            >
              <span class="inline-flex items-center gap-1">Created <ArrowUpDown class="w-3 h-3" /></span>
            </th>
            {#if isOwner}
              <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                Actions
              </th>
            {/if}
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200 dark:divide-slate-700">
          {#each paged.items as user (user.id)}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors">
              {#if isOwner}
                <td class="px-3 py-3">
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-slate-300 text-blue-600"
                    checked={selectedIds.has(user.id)}
                    onchange={() => toggleSelect(user.id)}
                  />
                </td>
              {/if}
              <td class="px-4 py-3 text-sm font-medium text-slate-900 dark:text-slate-100">
                {user.username}
              </td>
              <td class="px-4 py-3 text-sm text-slate-700 dark:text-slate-300">
                {user.displayName || '--'}
              </td>
              <td class="px-4 py-3 text-sm text-slate-700 dark:text-slate-300">
                {user.email || '--'}
              </td>
              <td class="px-4 py-3 text-sm">
                <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {user.roles?.includes('owner') ? 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300' : 'bg-slate-100 dark:bg-slate-700 text-slate-700 dark:text-slate-300'}">
                  {user.roles?.[0] || 'viewer'}
                </span>
              </td>
              <td class="px-4 py-3 text-sm">
                {#if user.disabled}
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300">
                    Disabled
                  </span>
                {:else}
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300">
                    Active
                  </span>
                {/if}
              </td>
              <td class="px-4 py-3 text-sm text-slate-500 dark:text-slate-400" title={user.createdAt}>
                {formatRelativeTime(user.createdAt)}
              </td>
              {#if isOwner}
                <td class="px-4 py-3">
                  <div class="flex items-center gap-1">
                    <button
                      type="button"
                      onclick={() => openEdit(user)}
                      title="Edit user"
                      class="p-1.5 rounded-md text-slate-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
                    >
                      <Pencil class="w-4 h-4" />
                    </button>
                    <button
                      type="button"
                      onclick={() => handleToggleDisable(user)}
                      title={user.disabled ? 'Enable user' : 'Disable user'}
                      class="p-1.5 rounded-md text-slate-400 hover:text-amber-600 hover:bg-amber-50 dark:hover:bg-amber-900/20 transition-colors"
                    >
                      {#if user.disabled}
                        <CheckCircle2 class="w-4 h-4" />
                      {:else}
                        <Ban class="w-4 h-4" />
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
<AdminUserFormDialog bind:open={formOpen} mode={formMode} user={formUser} onSuccess={handleFormSuccess} />
<DeleteConfirmDialog bind:open={deleteOpen} itemName={deleteTarget?.username ?? ''} onConfirm={handleDelete} />
<ResetPasswordDialog bind:open={resetPwdOpen} username={resetPwdTarget?.username ?? ''} onConfirm={handleResetPassword} />
<BulkConfirmDialog
  open={bulkAction !== null}
  action={bulkAction === 'delete' ? 'Delete' : bulkAction === 'disable' ? 'Disable' : 'Enable'}
  count={selectedCount}
  variant={bulkAction === 'delete' ? 'destructive' : 'default'}
  loading={bulkLoading}
  onConfirm={executeBulkAction}
  onCancel={() => { bulkAction = null; }}
/>
