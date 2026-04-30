<script lang="ts">
  import { onMount } from 'svelte';
  import { getWebUISettings, type WebUISettings } from '$lib/api';

  let loading = $state(true);
  let error = $state('');
  let settings = $state<WebUISettings | null>(null);

  const cookieMode = $derived(settings?.cookieSecure ? 'Secure cookies' : 'HTTP-compatible cookies');
  const requestTransport = $derived(settings?.transport === 'https' ? 'HTTPS' : 'HTTP');

  onMount(async () => {
    try {
      settings = await getWebUISettings();
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Unable to load Web UI settings.';
    } finally {
      loading = false;
    }
  });
</script>

<section class="page-stack">
  <div class="hero panel">
    <p class="eyebrow">Settings</p>
    <h2>Web UI transport</h2>
    <p class="muted">
      This page shows how the backend is currently configured for HTTP or HTTPS-oriented login.
      The setting is read-only in this slice and is controlled by the backend environment.
    </p>
  </div>

  {#if loading}
    <div class="panel">Loading Web UI settings…</div>
  {:else if error}
    <div class="panel error-text">{error}</div>
  {:else if settings}
    {#if settings.warning}
      <div class="callout warning">
        <strong>Transport warning</strong>
        <p>{settings.warning}</p>
      </div>
    {/if}

    <div class="grid cards">
      <article class="panel stat-card">
        <span>Cookie mode</span>
        <strong>{cookieMode}</strong>
        <small>{settings.cookieSecure ? 'Session cookies require HTTPS.' : 'Session cookies can be sent over HTTP.'}</small>
      </article>
      <article class="panel stat-card">
        <span>Current request</span>
        <strong>{requestTransport}</strong>
        <small>{settings.requestSecure ? 'The backend sees this request as encrypted.' : 'The backend sees this request as plain HTTP.'}</small>
      </article>
      <article class="panel stat-card">
        <span>HTTP login</span>
        <strong>{settings.httpLoginAllowed ? 'Allowed' : 'Blocked by Secure cookie'}</strong>
        <small>{settings.httpLoginAllowed ? 'Use only on trusted networks or for testing.' : 'Disable Secure cookies only for trusted HTTP use.'}</small>
      </article>
    </div>

    <section class="panel">
      <h3>Environment control</h3>
      <p class="muted">Restart the backend after changing this environment variable.</p>
      <dl class="definition-grid">
        <div>
          <dt>Allow HTTP login</dt>
          <dd><code>OCSERV_PANEL_COOKIE_SECURE=false</code></dd>
        </div>
        <div>
          <dt>HTTPS-oriented default</dt>
          <dd>Unset <code>OCSERV_PANEL_COOKIE_SECURE</code>, or set it to <code>true</code>.</dd>
        </div>
      </dl>
    </section>
  {:else}
    <div class="panel error-text">Web UI settings were not returned.</div>
  {/if}
</section>
