<script lang="ts">
  import { onMount } from 'svelte';
  import {
    getHistoryAnalytics,
    getMe,
    getSetupStatus,
    hasOwnerRole,
    principalFromAuth,
    purgeHistory,
    type HistoricalSession,
    type HistoryAnalytics,
    type Principal
  } from '$lib/api';

  let loading = $state(true);
  let error = $state('');
  let purgeError = $state('');
  let purgeSuccess = $state('');
  let setupRequired = $state(false);
  let principal = $state<Principal | null>(null);
  let analytics = $state<HistoryAnalytics | null>(null);
  let retentionDays = $state<30 | 90 | 180 | 365>(90);
  let confirmation = $state('');
  let purgeSubmitting = $state(false);

  const isOwner = $derived(hasOwnerRole(principal));
  const sessions = $derived(analytics?.recentSessions ?? analytics?.sessions ?? []);
  const sampleWarning = $derived(sampleDataWarning(analytics));
  const totals = $derived(calculateTotals(sessions, analytics?.summary));
  const purgeEnabled = $derived(isOwner && confirmation === 'PURGE' && !purgeSubmitting);

  onMount(() => {
    void loadHistory();
  });

  async function loadHistory() {
    loading = true;
    error = '';
    try {
      const setup = await getSetupStatus();
      setupRequired = setup.required;
      if (setup.required) return;

      principal = principalFromAuth(await getMe());
      analytics = await getHistoryAnalytics();
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Unable to load historical analytics.';
      principal = null;
    } finally {
      loading = false;
    }
  }

  async function handlePurge(event: SubmitEvent) {
    event.preventDefault();
    purgeError = '';
    purgeSuccess = '';
    if (!purgeEnabled) return;
    purgeSubmitting = true;
    try {
      await purgeHistory(retentionDays, 'PURGE');
      confirmation = '';
      purgeSuccess = 'History purge completed. Analytics were refreshed.';
      await loadHistory();
    } catch (caught) {
      purgeError = caught instanceof Error ? caught.message : 'History purge failed.';
    } finally {
      purgeSubmitting = false;
    }
  }

  function sampleDataWarning(value: HistoryAnalytics | null): string {
    const sampleData = value?.sampleData;
    if (typeof sampleData === 'boolean') return sampleData ? 'Sample historical sessions are present.' : '';
    if (sampleData?.enabled || sampleData?.present || Number(sampleData?.sessionCount ?? 0) > 0) {
      return sampleData?.message ?? 'Sample historical sessions are present.';
    }
    return '';
  }

  function calculateTotals(sessionsToTotal: HistoricalSession[], summary: Record<string, unknown> | undefined) {
    const sessionCount = numberFrom(summary?.sessionCount) ?? numberFrom(summary?.totalSessions) ?? sessionsToTotal.length;
    const rxBytes = numberFrom(summary?.rxBytes) ?? numberFrom(summary?.bytesIn) ?? sessionsToTotal.reduce((total, session) => total + (session.rxBytes ?? session.bytesIn ?? 0), 0);
    const txBytes = numberFrom(summary?.txBytes) ?? numberFrom(summary?.bytesOut) ?? sessionsToTotal.reduce((total, session) => total + (session.txBytes ?? session.bytesOut ?? 0), 0);
    return { sessionCount, rxBytes, txBytes, totalBytes: rxBytes + txBytes };
  }

  function numberFrom(value: unknown): number | null {
    return typeof value === 'number' && Number.isFinite(value) ? value : null;
  }

  function formatBytes(value: number): string {
    if (value < 1024) return `${value} B`;
    const units = ['KB', 'MB', 'GB', 'TB'];
    let scaled = value / 1024;
    let unit = units[0];
    for (let index = 1; scaled >= 1024 && index < units.length; index += 1) {
      scaled /= 1024;
      unit = units[index];
    }
    return `${scaled.toFixed(1)} ${unit}`;
  }

  function sessionUser(session: HistoricalSession): string {
    return session.username ?? session.user ?? '—';
  }
</script>

<section class="page-stack">
  <div class="hero panel">
    <p class="eyebrow">History</p>
    <h2>Historical session analytics</h2>
    <p class="muted">Analytics are read from the local backend API and persisted SQLite history.</p>
  </div>

  {#if loading}
    <div class="panel">Loading history…</div>
  {:else if setupRequired}
    <div class="panel callout warning">
      <h3>Initial setup required</h3>
      <p>Create the first owner before viewing historical analytics.</p>
      <a class="button" href="/setup">Set up owner</a>
    </div>
  {:else if !principal}
    <div class="panel callout">
      <h3>Sign in required</h3>
      <p>Historical analytics require a backend-confirmed session.</p>
      <a class="button" href="/login">Go to login</a>
      {#if error}<p class="error-text">{error}</p>{/if}
    </div>
  {:else if analytics}
    {#if sampleWarning}
      <div class="callout warning"><strong>Sample data warning:</strong> {sampleWarning}</div>
    {/if}

    <div class="grid cards">
      <article class="panel stat-card"><span>Sessions</span><strong>{totals.sessionCount}</strong></article>
      <article class="panel stat-card"><span>Received</span><strong>{formatBytes(totals.rxBytes)}</strong></article>
      <article class="panel stat-card"><span>Sent</span><strong>{formatBytes(totals.txBytes)}</strong></article>
      <article class="panel stat-card"><span>Total traffic</span><strong>{formatBytes(totals.totalBytes)}</strong></article>
    </div>

    <section class="panel">
      <h3>Recent historical sessions</h3>
      {#if sessions.length > 0}
        <div class="table-wrap">
          <table>
            <thead><tr><th>User</th><th>Started</th><th>Ended</th><th>Received</th><th>Sent</th><th>Remote</th></tr></thead>
            <tbody>
              {#each sessions as session}
                <tr>
                  <td>{sessionUser(session)}</td>
                  <td>{session.connectedAt ?? session.startedAt ?? '—'}</td>
                  <td>{session.disconnectedAt ?? session.endedAt ?? '—'}</td>
                  <td>{formatBytes(session.rxBytes ?? session.bytesIn ?? 0)}</td>
                  <td>{formatBytes(session.txBytes ?? session.bytesOut ?? 0)}</td>
                  <td>{session.remoteAddress ?? '—'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else}
        <p class="muted">No historical sessions returned yet.</p>
      {/if}
    </section>

    <section class="panel danger-panel">
      <h3>History retention purge</h3>
      {#if isOwner}
        <p class="muted">Destructive owner-only action. The backend enforces authorization and audit logging.</p>
        <form class="form-stack" onsubmit={handlePurge}>
          <label>
            Keep sessions newer than
            <select bind:value={retentionDays}>
              <option value={30}>30 days</option>
              <option value={90}>90 days</option>
              <option value={180}>180 days</option>
              <option value={365}>365 days</option>
            </select>
          </label>
          <label>
            Type PURGE to confirm deletion of older history
            <input bind:value={confirmation} autocomplete="off" />
          </label>
          {#if purgeError}<p class="error-text">{purgeError}</p>{/if}
          {#if purgeSuccess}<p class="success-text">{purgeSuccess}</p>{/if}
          <button class="button danger" type="submit" disabled={!purgeEnabled}>
            {purgeSubmitting ? 'Purging…' : 'Purge old history'}
          </button>
        </form>
      {:else}
        <p class="muted">Your role can view analytics but cannot purge historical records.</p>
      {/if}
    </section>
  {:else}
    <div class="panel error-text">{error || 'Historical analytics were not returned.'}</div>
  {/if}
</section>
