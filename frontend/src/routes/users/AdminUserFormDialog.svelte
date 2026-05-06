<script lang="ts">
  import { UserPlus, UserCog } from 'lucide-svelte';
  import { createAdminUser, updateAdminUser, type AdminUser } from '$lib/api';

  type Props = {
    open: boolean;
    mode: 'create' | 'edit';
    user?: AdminUser;
    onSuccess: () => void;
  };

  let { open = $bindable(), mode, user, onSuccess }: Props = $props();

  let username = $state('');
  let password = $state('');
  let displayName = $state('');
  let email = $state('');
  let role = $state('viewer');
  let submitting = $state(false);
  let error = $state('');

  // Reset form when dialog opens or user changes
  $effect(() => {
    if (open) {
      if (mode === 'edit' && user) {
        username = user.username;
        displayName = user.displayName || '';
        email = user.email || '';
        role = user.roles?.[0] || 'viewer';
        password = '';
      } else {
        username = '';
        password = '';
        displayName = '';
        email = '';
        role = 'viewer';
      }
      error = '';
    }
  });

  const validationError = $derived((() => {
    if (mode === 'create') {
      if (!username.trim()) return 'Username is required.';
      if (username.trim().length < 3) return 'Username must be at least 3 characters.';
      if (!password) return 'Password is required.';
      if (password.length < 8) return 'Password must be at least 8 characters.';
    }
    if (email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) return 'Invalid email format.';
    return '';
  })());

  const canSubmit = $derived(!validationError && !submitting && (mode === 'edit' || (username.trim() && password)));

  async function handleSubmit(e: Event) {
    e.preventDefault();
    if (!canSubmit) return;
    submitting = true;
    error = '';
    try {
      if (mode === 'create') {
        await createAdminUser({
          username: username.trim(),
          password,
          displayName: displayName.trim() || undefined,
          email: email.trim() || undefined,
          role
        });
      } else if (user) {
        await updateAdminUser(user.id, {
          displayName: displayName.trim() || undefined,
          email: email.trim() || undefined,
          role
        });
      }
      open = false;
      onSuccess();
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Operation failed.';
    } finally {
      submitting = false;
    }
  }

  function handleCancel() {
    error = '';
    open = false;
  }
</script>

{#if open}
<div class="fixed inset-0 z-50 flex items-center justify-center" role="dialog" aria-modal="true" aria-labelledby="admin-user-form-title">
  <button type="button" class="fixed inset-0 bg-black/50 cursor-default" onclick={handleCancel} aria-label="Close dialog"></button>
  <div class="relative bg-white dark:bg-slate-900 rounded-lg shadow-xl w-full max-w-md p-6 m-4">
    <div class="flex items-center gap-3 mb-4">
      <div class="w-10 h-10 rounded-full bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center flex-shrink-0">
        {#if mode === 'create'}
          <UserPlus class="w-5 h-5 text-blue-600 dark:text-blue-400" />
        {:else}
          <UserCog class="w-5 h-5 text-blue-600 dark:text-blue-400" />
        {/if}
      </div>
      <h2 id="admin-user-form-title" class="text-lg font-semibold text-slate-900 dark:text-slate-100">
        {mode === 'create' ? 'Create Web UI User' : 'Edit Web UI User'}
      </h2>
    </div>

    <form onsubmit={handleSubmit} class="space-y-4">
      <div>
        <label for="admin-username" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Username</label>
        {#if mode === 'create'}
          <input
            id="admin-username"
            type="text"
            bind:value={username}
            autocomplete="username"
            class="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            placeholder="Enter username"
          />
        {:else}
          <input
            id="admin-username"
            type="text"
            value={username}
            disabled
            class="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 cursor-not-allowed"
          />
        {/if}
      </div>

      {#if mode === 'create'}
        <div>
          <label for="admin-password" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Password</label>
          <input
            id="admin-password"
            type="password"
            bind:value={password}
            autocomplete="new-password"
            class="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            placeholder="Minimum 8 characters"
          />
        </div>
      {/if}

      <div>
        <label for="admin-display-name" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Display Name</label>
        <input
          id="admin-display-name"
          type="text"
          bind:value={displayName}
          class="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          placeholder="Optional display name"
        />
      </div>

      <div>
        <label for="admin-email" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Email</label>
        <input
          id="admin-email"
          type="email"
          bind:value={email}
          class="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          placeholder="Optional email"
        />
      </div>

      <div>
        <label for="admin-role" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Role</label>
        <select
          id="admin-role"
          bind:value={role}
          class="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
        >
          <option value="owner">Owner</option>
          <option value="viewer">Viewer</option>
        </select>
      </div>

      {#if validationError}
        <p class="text-sm text-amber-600 dark:text-amber-400">{validationError}</p>
      {/if}

      {#if error}
        <div class="px-3 py-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-sm text-red-700 dark:text-red-300" role="alert">
          {error}
        </div>
      {/if}

      <div class="flex justify-end gap-3 mt-6">
        <button
          type="button"
          onclick={handleCancel}
          disabled={submitting}
          class="px-4 py-2 bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-200 rounded-lg hover:bg-slate-300 dark:hover:bg-slate-600 disabled:opacity-50 text-sm font-medium transition-colors"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={!canSubmit}
          class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 text-sm font-medium transition-colors"
        >
          {#if submitting}
            {mode === 'create' ? 'Creating...' : 'Saving...'}
          {:else}
            {mode === 'create' ? 'Create User' : 'Save Changes'}
          {/if}
        </button>
      </div>
    </form>
  </div>
</div>
{/if}
