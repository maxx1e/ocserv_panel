<script lang="ts">
  import { UserPlus } from 'lucide-svelte';
  import { createVpnUser } from '$lib/api';

  type Props = {
    open: boolean;
    onSuccess: () => void;
  };

  let { open = $bindable(), onSuccess }: Props = $props();

  let username = $state('');
  let password = $state('');
  let confirmPassword = $state('');
  let submitting = $state(false);
  let error = $state('');

  // Reset form when dialog opens
  $effect(() => {
    if (open) {
      username = '';
      password = '';
      confirmPassword = '';
      error = '';
    }
  });

  const validationError = $derived((() => {
    if (!username.trim()) return '';
    if (username.trim().length < 2) return 'Username must be at least 2 characters.';
    if (password && password.length < 8) return 'Password must be at least 8 characters.';
    if (confirmPassword && password !== confirmPassword) return 'Passwords do not match.';
    return '';
  })());

  const canSubmit = $derived(
    username.trim().length >= 2 &&
    password.length >= 8 &&
    password === confirmPassword &&
    !submitting
  );

  async function handleSubmit(e: Event) {
    e.preventDefault();
    if (!canSubmit) return;
    submitting = true;
    error = '';
    try {
      await createVpnUser(username.trim(), password);
      open = false;
      onSuccess();
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Failed to create VPN user.';
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
<div class="fixed inset-0 z-50 flex items-center justify-center" role="dialog" aria-modal="true" aria-labelledby="vpn-user-form-title">
  <button type="button" class="fixed inset-0 bg-black/50 cursor-default" onclick={handleCancel} aria-label="Close dialog"></button>
  <div class="relative bg-white dark:bg-slate-900 rounded-lg shadow-xl w-full max-w-md p-6 m-4">
    <div class="flex items-center gap-3 mb-4">
      <div class="w-10 h-10 rounded-full bg-green-100 dark:bg-green-900/30 flex items-center justify-center flex-shrink-0">
        <UserPlus class="w-5 h-5 text-green-600 dark:text-green-400" />
      </div>
      <h2 id="vpn-user-form-title" class="text-lg font-semibold text-slate-900 dark:text-slate-100">Create VPN User</h2>
    </div>

    <form onsubmit={handleSubmit} class="space-y-4">
      <div>
        <label for="vpn-username" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Username</label>
        <input
          id="vpn-username"
          type="text"
          bind:value={username}
          autocomplete="username"
          class="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          placeholder="VPN username"
        />
      </div>

      <div>
        <label for="vpn-password" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Password</label>
        <input
          id="vpn-password"
          type="password"
          bind:value={password}
          autocomplete="new-password"
          class="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          placeholder="Minimum 8 characters"
        />
      </div>

      <div>
        <label for="vpn-confirm-password" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Confirm Password</label>
        <input
          id="vpn-confirm-password"
          type="password"
          bind:value={confirmPassword}
          autocomplete="new-password"
          class="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          placeholder="Re-enter password"
        />
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
          class="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50 text-sm font-medium transition-colors"
        >
          {submitting ? 'Creating...' : 'Create VPN User'}
        </button>
      </div>
    </form>
  </div>
</div>
{/if}
