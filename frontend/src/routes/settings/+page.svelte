<script lang="ts">
  import { onMount } from 'svelte';
  import {
    getSettingsSections,
    updateSettingByKey,
    getRestartStatus,
    triggerRestart,
    type SettingsSectionsResponse,
    type SettingDef,
    type ConfigStatus
  } from '$lib/api';
  import { setMode } from 'mode-watcher';
  import SettingsNav from '$lib/components/ui/SettingsNav.svelte';
  import RestartBanner from '$lib/components/ui/RestartBanner.svelte';
  import SettingField from '$lib/components/ui/SettingField.svelte';
  import CardSettings from '$lib/components/ui/CardSettings.svelte';

  let loading = $state(true);
  let error = $state('');
  let data = $state<SettingsSectionsResponse | null>(null);
  let savingKey = $state<string | null>(null);
  let restarting = $state(false);
  let restartError = $state('');
  let refreshWarning = $state('');

  let activeSection = $state('');
  const sectionIds = $derived(data?.sections.map((s) => s.id) ?? []);

  const configHealthy = $derived(data?.config_status === 'ok' || data?.config_status === undefined);

  // Build a label map from loaded sections for human-readable display in RestartBanner
  const labelMap = $derived.by(() => {
    const map = new Map<string, string>();
    for (const section of data?.sections ?? []) {
      for (const setting of section.settings) {
        map.set(setting.key, setting.label);
      }
    }
    return map;
  });

  let restartPollInterval: ReturnType<typeof setInterval> | null = null;
  let restartPollTimers: { interval?: ReturnType<typeof setInterval>; timeout?: ReturnType<typeof setTimeout> } = {};
  let observer: IntersectionObserver | null = null;

  // Set up IntersectionObserver reactively when sectionIds changes
  $effect(() => {
    if (observer) observer.disconnect();

    if (sectionIds.length === 0) return;

    observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            activeSection = entry.target.id;
          }
        }
      },
      { rootMargin: '-80px 0px -60% 0px', threshold: 0 }
    );

    requestAnimationFrame(() => {
      for (const id of sectionIds) {
        const el = document.getElementById(id);
        if (el) observer!.observe(el);
      }
    });

    return () => {
      if (observer) {
        observer.disconnect();
        observer = null;
      }
    };
  });

  onMount(() => {
    loadSettings();

    return () => {
      if (restartPollInterval) clearInterval(restartPollInterval);
      if (restartPollTimers.interval) clearInterval(restartPollTimers.interval);
      if (restartPollTimers.timeout) clearTimeout(restartPollTimers.timeout);
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

  async function handleUpdate(key: string, value: string): Promise<SettingDef> {
    savingKey = key;
    try {
      const result = await updateSettingByKey(key, value);
      // Apply theme immediately when the theme setting changes
      if (key === 'general.theme' && (value === 'light' || value === 'dark' || value === 'system')) {
        setMode(value);
      }
      // Refresh in background — don't let refresh failure affect the save result
      try {
        data = await getSettingsSections();
        refreshWarning = '';
      } catch (refreshErr) {
        console.warn('Failed to refresh settings after save:', refreshErr);
        refreshWarning = 'Setting saved successfully, but the page could not refresh. Reload to see the latest state.';
      }
      return result;
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
      restartPollTimers.interval = setInterval(async () => {
        try {
          const status = await getRestartStatus();
          if (!status.restart_required) {
            if (restartPollTimers.interval) clearInterval(restartPollTimers.interval);
            if (restartPollTimers.timeout) clearTimeout(restartPollTimers.timeout);
            window.location.reload();
          }
        } catch {
          // Keep polling
        }
      }, 3_000);

      // Safety: stop polling after 2 minutes
      restartPollTimers.timeout = setTimeout(() => {
        if (restartPollTimers.interval) clearInterval(restartPollTimers.interval);
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

  function configStatusTitle(status: ConfigStatus): string {
    switch (status) {
      case 'parse_error': return 'Configuration file has syntax errors';
      case 'permission_error': return 'Configuration file is not readable';
      case 'file_missing': return 'Configuration file not found';
      case 'read_error': return 'Unable to read configuration file';
      default: return 'Configuration issue';
    }
  }

  function configStatusInstruction(status: ConfigStatus): string {
    switch (status) {
      case 'parse_error': return 'Fix the syntax errors in the config file manually or restore from the .bak backup file, then reload this page.';
      case 'permission_error': return 'Check that the panel service user has read access to the config file, then reload this page.';
      case 'file_missing': return 'Ensure the config file exists at the expected path. The settings shown below are defaults and may not reflect your actual configuration.';
      case 'read_error': return 'An unexpected error occurred reading the config file. Check service logs for details.';
      default: return 'Check service logs for details.';
    }
  }
</script>

<div class="max-w-6xl mx-auto px-4 py-6">
  <!-- Page header -->
  <div class="mb-6">
    <h1 class="text-2xl font-semibold text-slate-900">Settings</h1>
    <p class="text-sm text-slate-500 mt-1">Manage ocserv and panel configuration.</p>
  </div>

  {#if loading}
    <div class="bg-card rounded-lg shadow-sm p-6 text-sm text-muted-foreground">Loading settings...</div>
  {:else if error}
    <div class="bg-card rounded-lg shadow-sm p-6 text-sm text-red-600">{error}</div>
  {:else if data}
    <!-- Two-column layout -->
    <div class="flex gap-6 items-start">
      <!-- Left sidebar: section nav (hidden on mobile) -->
      <aside class="hidden lg:block w-56 shrink-0 sticky top-20">
        <SettingsNav sections={data.sections} {activeSection} />
      </aside>

      <!-- Main content -->
      <div class="flex-1 min-w-0 flex flex-col gap-4">
        <!-- Mobile nav (tabs) — shown only on mobile -->
        <div class="lg:hidden">
          <SettingsNav sections={data.sections} {activeSection} mobile />
        </div>

        <!-- Config status alerts -->
        {#if data.config_status && data.config_status !== 'ok'}
          <div class="bg-amber-50 border border-amber-300 rounded-lg p-4">
            <div class="flex items-start gap-3">
              <svg class="w-5 h-5 text-amber-600 mt-0.5 shrink-0" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/>
                <path d="M12 9v4"/>
                <path d="M12 17h.01"/>
              </svg>
              <div class="flex-1 min-w-0">
                <p class="font-semibold text-amber-800">{configStatusTitle(data.config_status)}</p>
                {#if data.config_error}
                  <p class="text-sm text-amber-700 mt-1 font-mono bg-amber-100 rounded px-2 py-1 break-words">{data.config_error}</p>
                {/if}
                <p class="text-sm text-amber-700 mt-2">{configStatusInstruction(data.config_status)}</p>
                {#if !configHealthy}
                  <p class="text-xs text-amber-600 mt-2">Config-source settings are read-only until this issue is resolved.</p>
                {/if}
              </div>
            </div>
          </div>
        {/if}

        {#if restartError}
          <div class="bg-amber-50 border border-amber-200 rounded-lg p-4">
            <p class="font-semibold text-amber-800">Restart failed</p>
            <p class="text-sm text-amber-700 mt-1">{restartError}</p>
          </div>
        {/if}

        {#if refreshWarning}
          <div class="bg-blue-50 border border-blue-200 rounded-lg p-4">
            <p class="text-sm text-blue-700">{refreshWarning}</p>
            <button
              class="mt-2 text-sm text-blue-600 underline hover:text-blue-800"
              onclick={() => window.location.reload()}
            >Reload page</button>
          </div>
        {/if}

        <RestartBanner
          pendingChanges={data.pending_changes}
          onRestart={handleRestart}
          {restarting}
          {labelMap}
        />

        {#each data.sections as section (section.id)}
          <CardSettings
            id={section.id}
            title={section.title}
            label={section.label}
            description={section.description}
            stub={section.id === 'backup' || section.id === 'notifications'}
          >
            {#each section.settings as setting (setting.key)}
              <SettingField
                {setting}
                onUpdate={handleUpdate}
                {savingKey}
                configHealthy={configHealthy && section.id !== 'backup' && section.id !== 'notifications'}
              />
            {/each}
          </CardSettings>
        {/each}
      </div>
    </div>
  {:else}
    <div class="bg-card rounded-lg shadow-sm p-6 text-sm text-red-600">
      Settings data was not returned.
    </div>
  {/if}
</div>
