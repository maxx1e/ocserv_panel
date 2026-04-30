<script lang="ts">
  import { onMount } from 'svelte';
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

<section class="auth-card panel">
  <p class="eyebrow">Initial setup</p>
  <h2>Create the first owner</h2>
  <p class="muted">Owner setup calls the backend only and relies on its RBAC/bootstrap validation.</p>

  {#if setupRequired === null}
    <p>Checking setup status…</p>
  {:else if !setupRequired}
    <div class="callout">
      <strong>Setup is already complete.</strong>
      <p>Use the login form to start a backend-managed session.</p>
      <a class="button" href="/login">Go to login</a>
    </div>
  {:else}
    <form class="form-stack" onsubmit={handleSubmit}>
      <label>
        Owner username
        <input autocomplete="username" bind:value={username} required />
      </label>
      <label>
        Password
        <input autocomplete="new-password" type="password" bind:value={password} required />
      </label>
      <label>
        Confirm password
        <input autocomplete="new-password" type="password" bind:value={confirmPassword} required />
      </label>
      {#if passwordMismatch}<p class="error-text">Passwords must match before setup can continue.</p>{/if}
      {#if error}<p class="error-text">{error}</p>{/if}
      <button class="button" type="submit" disabled={!canSubmit}>
        {submitting ? 'Creating owner…' : 'Create owner'}
      </button>
    </form>
  {/if}
</section>
