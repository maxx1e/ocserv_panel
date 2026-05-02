<script lang="ts">
  import { onMount } from 'svelte';
  import {
    getSettingsSections,
    updateSettingByKey,
    getRestartStatus,
    triggerRestart,
    type SettingsSectionsResponse,
    type SettingsSection
  } from '$lib/api';
  import SettingsNav from '$lib/components/settings/SettingsNav.svelte';
  import RestartBanner from '$lib/components/settings/RestartBanner.svelte';
  import SettingsSectionComponent from '$lib/components/settings/SettingsSection.svelte';

  let loading = $state(true);
  let error = $state('');
  let data = $state<SettingsSectionsResponse | null>(null);
  let savingKey = $state<string | null>(null);
  let restarting = $state(false);
  let restartError = $state('');

  let activeSection = $state('');
  const sectionIds = $derived(data?.sections.map((s) => s.id) ?? []);

  let restartPollInterval: ReturnType<typeof setInterval> | null = null;

  onMount(() => {
    loadSettings();

    // Set up intersection observer for section tracking
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            activeSection = entry.target.id;
          }
        }
      },
      { rootMargin: '-80px 0px -60% 0px', threshold: 0 }
    );

    // Observe all sections after they render
    requestAnimationFrame(() => {
      for (const id of sectionIds) {
        const el = document.getElementById(id);
        if (el) observer.observe(el);
      }
    });

    return () => {
      observer.disconnect();
      if (restartPollInterval) clearInterval(restartPollInterval);
    };
  });

  // Poll restart status when restart is required
  $effect(() => {
    if (restartPollInterval) {
      clearInterval(restartPollInterval);
      restartPollInterval = null;
    }

    if (data?.restart_required && !restarting) {
      restartPollInterval = setInterval(() => {
        pollRestartStatus();
      }, 30_000);
    }

    return () => {
      if (restartPollInterval) {
        clearInterval(restartPollInterval);
        restartPollInterval = null;
      }
    };
  });

  async function loadSettings() {
    loading = true;
    error = '';
    try {
      data = await getSettingsSections();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Unable to load settings.';
    } finally {
      loading = false;
    }
  }

  async function handleUpdate(key: string, value: string) {
    savingKey = key;
    try {
      await updateSettingByKey(key, value);
      // Refresh settings to get updated state
      data = await getSettingsSections();
    } catch (e) {
      throw e;
    } finally {
      savingKey = null;
    }
  }

  async function pollRestartStatus() {
    try {
      const status = await getRestartStatus();
      if (!status.restart_required) {
        // Restart is no longer needed, refresh
        data = await getSettingsSections();
      }
    } catch {
      // Ignore polling errors
    }
  }

  async function handleRestart() {
    const confirmed = confirm(
      'Are you sure you want to restart the service? All active VPN sessions will be disconnected.'
    );
    if (!confirmed) return;

    restarting = true;
    restartError = '';

    // Poll status before sending restart
    await pollRestartStatus();

    try {
      await triggerRestart();
      // Poll for restart completion — page reloads when service is back
      const finalPoll = setInterval(async () => {
        try {
          const status = await getRestartStatus();
          if (!status.restart_required) {
            clearInterval(finalPoll);
            window.location.reload();
          }
        } catch {
          // Keep polling
        }
      }, 3_000);

      // Safety: stop polling after 2 minutes
      setTimeout(() => {
        clearInterval(finalPoll);
        restarting = false;
        restartError =
          'The service did not come back within 2 minutes. Check the service manually.';
      }, 120_000);
    } catch (e) {
      restarting = false;
      restartError =
        e instanceof Error ? e.message : 'Restart failed. Check service logs.';
    }
  }
</script>

<section class="page-stack">
  <div class="hero panel">
    <p class="eyebrow">Configure</p>
    <h2>Settings</h2>
    <p class="muted">
      Adjust ocserv and panel configuration. Changes to config-source settings require a service
      restart; database-source changes take effect immediately.
    </p>
  </div>

  {#if loading}
    <div class="panel">Loading settings…</div>
  {:else if error}
    <div class="panel error-text">{error}</div>
  {:else if data}
    {#if restartError}
      <div class="callout warning">
        <strong>Restart failed</strong>
        <p>{restartError}</p>
      </div>
    {/if}

    <RestartBanner
      pendingChanges={data.pending_changes}
      onRestart={handleRestart}
      {restarting}
    />

    <SettingsNav
      sections={data.sections}
      {activeSection}
    />

    {#each data.sections as section (section.id)}
      <SettingsSectionComponent
        {section}
        onUpdate={handleUpdate}
        {savingKey}
      />
    {/each}
  {:else}
    <div class="panel error-text">Settings data was not returned.</div>
  {/if}
</section>
