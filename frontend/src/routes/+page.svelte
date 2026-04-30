<script lang="ts">
  import { onMount } from 'svelte';
  import { getDashboardSummary, getMe, getSetupStatus, principalFromAuth, type DashboardSummary, type Principal } from '$lib/api';

  let loading = $state(true);
  let error = $state('');
  let setupRequired = $state(false);
  let principal = $state<Principal | null>(null);
  let summary = $state<DashboardSummary | null>(null);

  const sampleDataMessage = $derived(sampleDataText(summary?.sampleData));
  const recentAudit = $derived(summary?.recentAudit ?? []);
  const analyticsRows = $derived(summary?.analytics ? Object.entries(summary.analytics) : []);

  onMount(() => {
    void loadDashboard();
  });

  async function loadDashboard() {
    loading = true;
    error = '';
    try {
      const setup = await getSetupStatus();
      setupRequired = setup.required;
      if (setup.required) return;

      principal = principalFromAuth(await getMe());
      summary = await getDashboardSummary();
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Unable to load dashboard state.';
      principal = null;
    } finally {
      loading = false;
    }
  }

  function sampleDataText(value: DashboardSummary['sampleData']): string {
    if (typeof value === 'boolean') return value ? 'Sample data is present for baseline visibility.' : 'No sample data reported.';
    if (value?.enabled || value?.present) return value.message ?? 'Sample data is present for baseline visibility.';
    return 'No sample data reported.';
  }

  function formatValue(value: unknown): string {
    if (value === null || value === undefined) return '—';
    if (typeof value === 'object') return JSON.stringify(value);
    return String(value);
  }
</script>

<section class="page-stack">
  <div class="hero panel">
    <p class="eyebrow">Dashboard</p>
    <h2>Local ocserv administration</h2>
    <p class="muted">
      This UI reads same-origin backend APIs. It does not access local files, sockets, commands,
      certificates, or secrets directly.
    </p>
  </div>

  {#if loading}
    <div class="panel">Loading dashboard state…</div>
  {:else if setupRequired}
    <div class="panel callout warning">
      <h3>Initial setup required</h3>
      <p>Create the first owner before viewing privileged dashboard data.</p>
      <a class="button" href="/setup">Set up owner</a>
    </div>
  {:else if !principal}
    <div class="panel callout">
      <h3>Sign in required</h3>
      <p>Dashboard data is available only after the backend confirms an HttpOnly session cookie.</p>
      <a class="button" href="/login">Go to login</a>
      {#if error}<p class="error-text">{error}</p>{/if}
    </div>
  {:else if summary}
    <div class="grid cards">
      <article class="panel stat-card">
        <span>Backend status</span>
        <strong>{summary.backendStatus ?? 'unknown'}</strong>
      </article>
      <article class="panel stat-card">
        <span>ocserv status</span>
        <strong>{summary.ocservStatus ?? 'not_configured'}</strong>
        <small>Local-only baseline; no live ocserv control is active.</small>
      </article>
      <article class="panel stat-card">
        <span>Active sessions</span>
        <strong>{summary.activeSessionCount ?? 0}</strong>
      </article>
      <article class="panel stat-card">
        <span>Sample data</span>
        <strong>{sampleDataMessage}</strong>
      </article>
    </div>

    <section class="panel">
      <h3>Analytics summary</h3>
      {#if analyticsRows.length > 0}
        <dl class="definition-grid">
          {#each analyticsRows as [key, value]}
            <div>
              <dt>{key}</dt>
              <dd>{formatValue(value)}</dd>
            </div>
          {/each}
        </dl>
      {:else}
        <p class="muted">No analytics summary returned yet.</p>
      {/if}
    </section>

    <section class="panel">
      <h3>Recent audit</h3>
      {#if recentAudit.length > 0}
        <div class="table-wrap">
          <table>
            <thead><tr><th>Time</th><th>Actor</th><th>Action</th><th>Result</th></tr></thead>
            <tbody>
              {#each recentAudit as event}
                <tr>
                  <td>{event.createdAt ?? event.timestamp ?? '—'}</td>
                  <td>{event.actorUsername ?? event.actor ?? '—'}</td>
                  <td>{event.action ?? event.message ?? '—'}</td>
                  <td>{event.outcome ?? event.result ?? '—'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else}
        <p class="muted">No audit events returned yet.</p>
      {/if}
    </section>
  {:else}
    <div class="panel error-text">{error || 'Dashboard summary was not returned.'}</div>
  {/if}
</section>
