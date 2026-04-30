<script lang="ts">
  import { onMount } from 'svelte';
  import { getSetupStatus, login } from '$lib/api';

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let setupRequired = $state(false);
  let submitting = $state(false);

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

<section class="auth-card panel">
  <p class="eyebrow">Login</p>
  <h2>Admin sign in</h2>
  <p class="muted">The backend sets an HttpOnly session cookie. This page never stores tokens or passwords.</p>

  {#if setupRequired}
    <div class="callout warning">
      <strong>Initial setup is required.</strong>
      <p>Create the first owner account before logging in.</p>
      <a class="button" href="/setup">Set up owner</a>
    </div>
  {:else}
    <form class="form-stack" onsubmit={handleSubmit}>
      <label>
        Username
        <input autocomplete="username" bind:value={username} required />
      </label>
      <label>
        Password
        <input autocomplete="current-password" type="password" bind:value={password} required />
      </label>
      {#if error}<p class="error-text">{error}</p>{/if}
      <button class="button" type="submit" disabled={submitting || !username.trim() || !password}>
        {submitting ? 'Signing in…' : 'Sign in'}
      </button>
    </form>
  {/if}
</section>
