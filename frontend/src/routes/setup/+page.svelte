<script lang="ts">
  import { onMount } from 'svelte';
  import { Shield } from 'lucide-svelte';
  import { getSetupStatus, setupOwner } from '$lib/api';

  let username = $state('');
  let password = $state('');
  let confirmPassword = $state('');
  let setupRequired = $state<boolean | null>(null);
  let error = $state('');
  let submitting = $state(false);

  const passwordMismatch = $derived(password.length > 0 && confirmPassword.length > 0 && password !== confirmPassword);
  const canSubmit = $derived(username.trim().length > 0 && password.length > 0 && password === confirmPassword && !submitting);

  onMount(async () => {
    try {
      setupRequired = (await getSetupStatus()).required;
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Unable to reach setup status.';
    }
  });

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    if (!canSubmit) return;
    submitting = true;
    const submittedPassword = password;
    try {
      await setupOwner(username.trim(), submittedPassword);
      password = '';
      confirmPassword = '';
      window.location.href = '/';
    } catch (caught) {
      password = '';
      confirmPassword = '';
      error = caught instanceof Error ? caught.message : 'Owner setup failed.';
    } finally {
      submitting = false;
    }
  }
</script>

<div class="max-w-md mx-auto mt-20">
  <div class="bg-white rounded-lg shadow-sm p-8">
    <!-- Icon -->
    <div class="flex justify-center mb-6">
      <div class="w-12 h-12 rounded-full bg-slate-100 flex items-center justify-center">
        <Shield class="w-6 h-6 text-slate-600" />
      </div>
    </div>

    <!-- Header -->
    <p class="text-xs font-semibold text-slate-400 uppercase tracking-wider text-center">Initial setup</p>
    <h2 class="mt-1 text-xl font-bold text-slate-700 text-center">Create the first owner</h2>
    <p class="mt-2 text-sm text-slate-500 text-center">Owner setup calls the backend only and relies on its RBAC/bootstrap validation.</p>

    <!-- States -->
    {#if setupRequired === null}
      <p class="mt-6 text-sm text-slate-500 text-center">Checking setup status…</p>
    {:else if !setupRequired}
      <div class="mt-6 bg-blue-50 border border-blue-200 rounded-lg p-4">
        <strong class="text-blue-900 text-sm">Setup is already complete.</strong>
        <p class="mt-1 text-sm text-blue-800">Use the login form to start a backend-managed session.</p>
        <a class="inline-block mt-3 bg-slate-600 hover:bg-slate-700 text-white font-semibold rounded-lg px-4 py-2 text-sm" href="/login">
          Go to login
        </a>
      </div>
    {:else}
      <form class="mt-6 space-y-4" onsubmit={handleSubmit}>
        <label class="block">
          <span class="block text-sm font-medium text-slate-500">Owner username</span>
          <input
            class="mt-1 block w-full rounded-lg"
            type="text"
            autocomplete="username"
            bind:value={username}
            required
          />
        </label>
        <label class="block">
          <span class="block text-sm font-medium text-slate-500">Password</span>
          <input
            class="mt-1 block w-full rounded-lg"
            type="password"
            autocomplete="new-password"
            bind:value={password}
            required
          />
        </label>
        <label class="block">
          <span class="block text-sm font-medium text-slate-500">Confirm password</span>
          <input
            class="mt-1 block w-full rounded-lg"
            type="password"
            autocomplete="new-password"
            bind:value={confirmPassword}
            required
          />
        </label>
        {#if passwordMismatch}<p class="text-red-600 text-sm">Passwords must match before setup can continue.</p>{/if}
        {#if error}<p class="text-red-600 text-sm">{error}</p>{/if}
        <button
          class="w-full bg-slate-600 hover:bg-slate-700 text-white font-semibold rounded-lg px-4 py-2 text-sm disabled:opacity-50 disabled:cursor-not-allowed"
          type="submit"
          disabled={!canSubmit}
        >
          {submitting ? 'Creating owner…' : 'Create owner'}
        </button>
      </form>
    {/if}
  </div>
</div>
