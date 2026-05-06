<script lang="ts">
  import { KeyRound } from 'lucide-svelte';

  type Props = {
    open: boolean;
    username: string;
    onConfirm: (password: string) => Promise<void>;
  };

  let { open = $bindable(), username, onConfirm }: Props = $props();

  let password = $state('');
  let confirmPassword = $state('');
  let submitting = $state(false);
  let error = $state('');

  const validationError = $derived((() => {
    if (!password) return '';
    if (password.length < 8) return 'Password must be at least 8 characters.';
    if (confirmPassword && password !== confirmPassword) return 'Passwords do not match.';
    return '';
  })());

  const canSubmit = $derived(password.length >= 8 && password === confirmPassword && !submitting);

  async function handleSubmit(e: Event) {
    e.preventDefault();
    if (!canSubmit) return;
    submitting = true;
    error = '';
    try {
      await onConfirm(password);
      password = '';
      confirmPassword = '';
      open = false;
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Password reset failed.';
    } finally {
      submitting = false;
    }
  }

  function handleCancel() {
    password = '';
    confirmPassword = '';
    error = '';
    open = false;
  }
</script>

{#if open}
<div class="fixed inset-0 z-50 flex items-center justify-center" role="dialog" aria-modal="true" aria-labelledby="reset-pwd-title">
  <button type="button" class="fixed inset-0 bg-black/50 cursor-default" onclick={handleCancel} aria-label="Close dialog"></button>
  <div class="relative bg-white dark:bg-slate-900 rounded-lg shadow-xl w-full max-w-md p-6 m-4">
    <div class="flex items-center gap-3 mb-4">
      <div class="w-10 h-10 rounded-full bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center flex-shrink-0">
        <KeyRound class="w-5 h-5 text-blue-600 dark:text-blue-400" />
      </div>
      <div>
        <h2 id="reset-pwd-title" class="text-lg font-semibold text-slate-900 dark:text-slate-100">Reset password</h2>
        <p class="text-sm text-slate-500 dark:text-slate-400">For user: {username}</p>
      </div>
    </div>

    <form onsubmit={handleSubmit} class="space-y-4">
      <div>
        <label for="new-password" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">New password</label>
        <input
          id="new-password"
          type="password"
          bind:value={password}
          autocomplete="new-password"
          class="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          placeholder="Minimum 8 characters"
        />
      </div>

      <div>
        <label for="confirm-password" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Confirm password</label>
        <input
          id="confirm-password"
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
          class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 text-sm font-medium transition-colors"
        >
          {submitting ? 'Resetting...' : 'Reset Password'}
        </button>
      </div>
    </form>
  </div>
</div>
{/if}
