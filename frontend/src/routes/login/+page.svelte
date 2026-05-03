<script lang="ts">
  import { onMount } from 'svelte';
  import { Lock } from 'lucide-svelte';
  import { getSetupStatus, getWebUISettings, login, type WebUISettings } from '$lib/api';

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let setupRequired = $state(false);
  let submitting = $state(false);
  let webUISettings = $state<WebUISettings | null>(null);
  const transportWarning = $derived(webUISettingsWarning(webUISettings));

  onMount(async () => {
    try {
      const [setupStatus, settings] = await Promise.all([getSetupStatus(), getWebUISettings()]);
      setupRequired = setupStatus.required;
      webUISettings = settings;
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Unable to reach setup status.';
    }
  });

  function webUISettingsWarning(settings: WebUISettings | null): string {
    if (!settings) return '';
    if (settings.warning) return settings.warning;
    if (settings.httpLoginAllowed || settings.transport === 'http') {
      return 'HTTP login sends credentials/session cookies without transport encryption. Use only on trusted local networks or for testing.';
    }
    return '';
  }

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    submitting = true;
    const submittedPassword = password;
    try {
      await login(username.trim(), submittedPassword);
      password = '';
      window.location.href = '/';
    } catch (caught) {
      password = '';
      error = caught instanceof Error ? caught.message : 'Login failed.';
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
        <Lock class="w-6 h-6 text-slate-600" />
      </div>
    </div>

    <!-- Header -->
    <p class="text-xs font-semibold text-slate-400 uppercase tracking-wider text-center">Login</p>
    <h2 class="mt-1 text-xl font-bold text-slate-700 text-center">Admin sign in</h2>
    <p class="mt-2 text-sm text-slate-500 text-center">The backend sets an HttpOnly session cookie. This page never stores tokens or passwords.</p>

    <!-- Transport warning -->
    {#if transportWarning}
      <div class="mt-6 bg-amber-50 border border-amber-200 rounded-lg p-4">
        <strong class="text-amber-900 text-sm">Insecure login warning</strong>
        <p class="mt-1 text-sm text-amber-800">{transportWarning}</p>
      </div>
    {/if}

    <!-- Setup required or Login form -->
    {#if setupRequired}
      <div class="mt-6 bg-amber-50 border border-amber-200 rounded-lg p-4">
        <strong class="text-amber-900 text-sm">Initial setup is required.</strong>
        <p class="mt-1 text-sm text-amber-800">Create the first owner account before logging in.</p>
        <a class="inline-block mt-3 bg-slate-600 hover:bg-slate-700 text-white font-semibold rounded-lg px-4 py-2 text-sm" href="/setup">
          Set up owner
        </a>
      </div>
    {:else}
      <form class="mt-6 space-y-4" onsubmit={handleSubmit}>
        <label class="block">
          <span class="block text-sm font-medium text-slate-500">Username</span>
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
            autocomplete="current-password"
            bind:value={password}
            required
          />
        </label>
        {#if error}<p class="text-red-600 text-sm">{error}</p>{/if}
        <button
          class="w-full bg-slate-600 hover:bg-slate-700 text-white font-semibold rounded-lg px-4 py-2 text-sm disabled:opacity-50 disabled:cursor-not-allowed"
          type="submit"
          disabled={submitting || !username.trim() || !password}
        >
          {submitting ? 'Signing in…' : 'Sign in'}
        </button>
      </form>
    {/if}
  </div>
</div>
