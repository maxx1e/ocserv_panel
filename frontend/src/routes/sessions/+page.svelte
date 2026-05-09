<script lang="ts">
  import { onMount } from 'svelte';
  import { X, AlertTriangle } from 'lucide-svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import RefreshTimer from './RefreshTimer.svelte';
  import SessionStatusCards from './SessionStatusCards.svelte';
  import SessionTabs from './SessionTabs.svelte';
  import ActiveSessionsTab from './ActiveSessionsTab.svelte';
  import AllSessionsTab from './AllSessionsTab.svelte';
  import EventsTab from './EventsTab.svelte';
  import RoutesTab from './RoutesTab.svelte';
  import BansTab from './BansTab.svelte';
  import DisconnectUserDialog from './DisconnectUserDialog.svelte';
  import ReadinessDiagnostic from './ReadinessDiagnostic.svelte';
  import {
    getServerStatus,
    getMe,
    getSetupStatus,
    getOcservReadiness,
    disconnectOcservSession,
    terminateOcservSession,
    disconnectUser,
    principalFromAuth,
    hasOwnerRole,
    ApiError,
    type ServerStatus,
    type OcservReadiness,
    type Principal
  } from '$lib/api';

  // ── page state ─────────────────────────────────────────────────
  let pageLoading = $state(true);
  let pageError = $state('');
  let notConfigured = $state(false);
  let forbidden = $state(false);
  let setupRequired = $state(false);
  let principal = $state<Principal | null>(null);

  // ── readiness diagnostics ──────────────────────────────────────
  let readinessData = $state<OcservReadiness | null>(null);
  let readinessLoading = $state(false);

  // ── server status ──────────────────────────────────────────────
  let serverStatus = $state<ServerStatus | null>(null);
  let statusLoading = $state(true);
  let statusError = $state('');

  // ── tabs ───────────────────────────────────────────────────────
  type TabId = 'active' | 'all' | 'events' | 'routes' | 'bans';
  let activeTab = $state<TabId>('active');
  let refreshSignal = $state(0);

  // ── banners ────────────────────────────────────────────────────
  let successBanner = $state('');
  let errorBanner = $state('');

  // ── confirm modals ─────────────────────────────────────────────
  type SessionAction = { type: 'disconnect' | 'terminate'; sessionId: string; username: string } | null;
  let pendingSessionAction = $state<SessionAction>(null);
  let sessionActionError = $state('');
  let sessionActionSubmitting = $state(false);

  let disconnectUserTarget = $state<string | null>(null);
  let disconnectUserError = $state('');
  let disconnectUserSubmitting = $state(false);

  // ── derived ────────────────────────────────────────────────────
  const isOwner = $derived(hasOwnerRole(principal));
  const timerPaused = $derived(pendingSessionAction !== null || disconnectUserTarget !== null);

  const tabs = $derived([
    { id: 'active', label: 'Active Users' },
    { id: 'all', label: 'All Sessions' },
    { id: 'events', label: 'Events' },
    { id: 'routes', label: 'Routes' },
    { id: 'bans', label: 'Bans', badge: serverStatus?.bannedIps ?? 0 }
  ]);

  // ── lifecycle ──────────────────────────────────────────────────
  onMount(() => {
    const hash = window.location.hash.replace('#', '');
    if (['active', 'all', 'events', 'routes', 'bans'].includes(hash)) {
      activeTab = hash as TabId;
    }
    void initPage();
  });

  // ── data ───────────────────────────────────────────────────────
  async function initPage() {
    pageLoading = true;
    pageError = '';
    notConfigured = false;
    forbidden = false;
    setupRequired = false;
    try {
      const setup = await getSetupStatus();
      if (setup.required) { setupRequired = true; return; }
      principal = principalFromAuth(await getMe());
      await loadServerStatus();
    } catch (caught) {
      if (caught instanceof ApiError) {
        if (caught.status === 503) {
          notConfigured = true;
          void loadReadiness();
          return;
        }
        if (caught.status === 403) { forbidden = true; return; }
      }
      pageError = caught instanceof Error ? caught.message : 'Unable to load sessions page.';
    } finally {
      pageLoading = false;
    }
  }

  async function loadReadiness() {
    readinessLoading = true;
    try {
      readinessData = await getOcservReadiness();
    } catch {
      readinessData = null;
    } finally {
      readinessLoading = false;
    }
  }

  async function handleStartService() {
    try {
      await fetch('/api/ocserv/service/start', { method: 'POST', credentials: 'same-origin' });
      // Retry loading after a short delay
      setTimeout(() => void initPage(), 2000);
    } catch {
      // Errors will show in next readiness check
    }
  }

  async function loadServerStatus() {
    statusLoading = true;
    statusError = '';
    try {
      serverStatus = await getServerStatus();
    } catch (caught) {
      statusError = caught instanceof Error ? caught.message : 'Status unavailable';
    } finally {
      statusLoading = false;
    }
  }

  async function refreshData() {
    await loadServerStatus();
    refreshSignal++;
  }

  function handleTabChange(tabId: string) {
    activeTab = tabId as TabId;
    window.location.hash = tabId;
  }

  // ── session actions (disconnect/terminate by ID) ───────────────
  function confirmDisconnectSession(session: { id: string; username: string }) {
    pendingSessionAction = { type: 'disconnect', sessionId: session.id, username: session.username };
    sessionActionError = '';
  }

  function confirmTerminateSession(session: { id: string; username: string }) {
    pendingSessionAction = { type: 'terminate', sessionId: session.id, username: session.username };
    sessionActionError = '';
  }

  function cancelSessionAction() {
    pendingSessionAction = null;
    sessionActionError = '';
  }

  async function executeSessionAction() {
    if (!pendingSessionAction || sessionActionSubmitting) return;
    sessionActionSubmitting = true;
    sessionActionError = '';
    const { type, sessionId, username } = pendingSessionAction;
    try {
      if (type === 'disconnect') {
        await disconnectOcservSession(sessionId);
      } else {
        await terminateOcservSession(sessionId);
      }
      const verb = type === 'disconnect' ? 'disconnected' : 'terminated';
      successBanner = `Session for ${username} ${verb}.`;
      pendingSessionAction = null;
      refreshSignal++;
    } catch (caught) {
      sessionActionError = caught instanceof Error ? caught.message : `Failed to ${type} session.`;
    } finally {
      sessionActionSubmitting = false;
    }
  }

  // ── disconnect user (all sessions) ─────────────────────────────
  function confirmDisconnectUser(username: string) {
    disconnectUserTarget = username;
    disconnectUserError = '';
  }

  function cancelDisconnectUser() {
    disconnectUserTarget = null;
    disconnectUserError = '';
  }

  async function executeDisconnectUser() {
    if (!disconnectUserTarget || disconnectUserSubmitting) return;
    disconnectUserSubmitting = true;
    disconnectUserError = '';
    try {
      await disconnectUser(disconnectUserTarget);
      successBanner = `All sessions for ${disconnectUserTarget} disconnected.`;
      disconnectUserTarget = null;
      refreshSignal++;
    } catch (caught) {
      disconnectUserError = caught instanceof Error ? caught.message : 'Failed to disconnect user.';
    } finally {
      disconnectUserSubmitting = false;
    }
  }
</script>

{#if pageLoading}
  <div class="flex items-center justify-center py-16">
    <div class="text-center">
      <div class="w-8 h-8 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin mx-auto mb-3"></div>
      <p class="text-slate-500 text-sm">Loading sessions...</p>
    </div>
  </div>

{:else if setupRequired}
  <div class="bg-amber-50 border border-amber-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-amber-800">Initial setup required</h3>
    <p class="text-amber-700 text-sm mt-1">Create the first owner before viewing active sessions.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-amber-600 text-white text-sm font-medium rounded-md hover:bg-amber-700 transition-colors" href="/setup">
      Set up owner
    </a>
  </div>

{:else if notConfigured}
  <ReadinessDiagnostic
    readiness={readinessData}
    loading={readinessLoading}
    isOwner={hasOwnerRole(principal)}
    onStartService={handleStartService}
  />

{:else if forbidden}
  <EmptyState
    icon={AlertTriangle}
    title="Access denied"
    description="Your role cannot view sessions."
  />

{:else if !principal}
  <div class="bg-slate-50 border border-slate-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-slate-700">Sign in required</h3>
    <p class="text-slate-500 text-sm mt-1">Active session data requires authentication.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-slate-700 text-white text-sm font-medium rounded-md hover:bg-slate-800 transition-colors" href="/login">
      Go to login
    </a>
    {#if pageError}
      <p class="mt-3 text-sm text-red-600">{pageError}</p>
    {/if}
  </div>

{:else}
  <div class="space-y-4">
    <!-- Banners -->
    {#if errorBanner}
      <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-red-50 text-red-800" role="alert">
        <span>{errorBanner}</span>
        <button type="button" onclick={() => (errorBanner = '')} class="ml-4 text-red-600 hover:text-red-800" aria-label="Dismiss">
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}
    {#if successBanner}
      <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-green-50 text-green-800" role="status">
        <span>{successBanner}</span>
        <button type="button" onclick={() => (successBanner = '')} class="ml-4 text-green-600 hover:text-green-800" aria-label="Dismiss">
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}

    <!-- Page header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <h1 class="text-2xl font-bold text-slate-800">VPN Sessions</h1>
      <RefreshTimer onRefresh={refreshData} paused={timerPaused} />
    </div>

    <!-- Status cards -->
    <SessionStatusCards status={serverStatus} loading={statusLoading} error={statusError} />

    <!-- Tabs -->
    <SessionTabs {tabs} {activeTab} onTabChange={handleTabChange} />

    <!-- Tab content -->
    <div role="tabpanel" id="tabpanel-{activeTab}" aria-labelledby="tab-{activeTab}">
      {#if activeTab === 'active'}
        <ActiveSessionsTab
          {isOwner}
          {refreshSignal}
          onDisconnectSession={confirmDisconnectSession}
          onTerminateSession={confirmTerminateSession}
          onDisconnectUser={confirmDisconnectUser}
        />
      {:else if activeTab === 'all'}
        <AllSessionsTab {refreshSignal} />
      {:else if activeTab === 'events'}
        <EventsTab {refreshSignal} />
      {:else if activeTab === 'routes'}
        <RoutesTab {refreshSignal} />
      {:else if activeTab === 'bans'}
        <BansTab {refreshSignal} {isOwner} />
      {/if}
    </div>
  </div>
{/if}

<!-- Session disconnect/terminate modal -->
{#if pendingSessionAction !== null}
  <div
    class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
    role="dialog"
    aria-modal="true"
    aria-labelledby="session-action-title"
  >
    <div class="bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
      <h2 id="session-action-title" class="text-lg font-semibold text-slate-800">
        {pendingSessionAction.type === 'disconnect' ? 'Disconnect session?' : 'Terminate session?'}
      </h2>
      <p class="text-sm text-slate-600 mt-2">
        {#if pendingSessionAction.type === 'disconnect'}
          This will drop the current connection for <strong>{pendingSessionAction.username}</strong>. The user may reconnect if their session cookie is still valid.
        {:else}
          This will drop the connection and invalidate the session cookie for <strong>{pendingSessionAction.username}</strong>. The user must log in again.
        {/if}
      </p>
      {#if sessionActionError}
        <div class="mt-3 px-3 py-2 bg-red-50 rounded text-sm text-red-700" role="alert">{sessionActionError}</div>
      {/if}
      <div class="flex items-center justify-end gap-3 mt-6">
        <button
          type="button"
          onclick={cancelSessionAction}
          disabled={sessionActionSubmitting}
          class="px-4 py-2 text-sm font-medium rounded-md border border-slate-200 text-slate-600 bg-white hover:bg-slate-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={executeSessionAction}
          disabled={sessionActionSubmitting}
          class={pendingSessionAction.type === 'disconnect'
            ? 'px-4 py-2 text-sm font-medium rounded-md bg-orange-600 text-white hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors'
            : 'px-4 py-2 text-sm font-medium rounded-md bg-red-600 text-white hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors'}
        >
          {#if sessionActionSubmitting}
            {pendingSessionAction.type === 'disconnect' ? 'Disconnecting...' : 'Terminating...'}
          {:else}
            {pendingSessionAction.type === 'disconnect' ? 'Disconnect' : 'Terminate'}
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Disconnect user dialog -->
{#if disconnectUserTarget}
  <DisconnectUserDialog
    username={disconnectUserTarget}
    submitting={disconnectUserSubmitting}
    error={disconnectUserError}
    onConfirm={executeDisconnectUser}
    onCancel={cancelDisconnectUser}
  />
{/if}
