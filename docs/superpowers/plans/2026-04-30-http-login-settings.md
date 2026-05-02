# HTTP Login Settings Visibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the current Web UI HTTP/HTTPS login posture in the Settings tab and warn users on the login page when HTTP-compatible login is enabled or the page is accessed over HTTP.

**Architecture:** Keep `OCSERV_PANEL_COOKIE_SECURE` as the only source of truth for HTTP-compatible login. Add a read-only backend settings endpoint, consume it from SvelteKit Settings/Login UI, and document the security tradeoff without adding TLS, HAProxy, cert, systemd, or live ocserv mutation.

**Tech Stack:** Go `net/http` backend, existing auth/session cookie wiring, SvelteKit/Svelte 5 frontend, existing Markdown operations/security docs, SQLite unchanged.

---

## Coordination Rules

- Work in `/opt/git/ocserv_panel` or a fresh worktree created from current `master` before implementation.
- Use `PATH="$HOME/.local/bin:$PATH"` for Go and Node commands.
- Do not run `ocserv`, `occtl`, `ocpasswd`, `systemctl`, `sudo`, service-control commands, HAProxy commands, certificate commands, or destructive filesystem commands.
- Do not add external services, hosted dependencies, CORS, cloud control planes, queues, or observability stacks.
- Do not store plaintext passwords, raw session tokens, private keys, credentials, or generated secrets in source or SQLite.
- Do not commit unless the user explicitly asks for a commit.
- Use subagents sequentially because backend API, frontend API contract, and docs share a single API contract.

## File Map

- `backend/internal/server/server.go`: add read-only `/api/settings/webui` route, response type, request transport detection, and warning message generation.
- `backend/internal/server/server_test.go`: add endpoint tests for default secure-cookie mode, HTTP-compatible cookie mode, HTTPS request detection, and secret-free response shape.
- `frontend/src/lib/api.ts`: add `WebUISettings` type and `getWebUISettings()` helper.
- `frontend/src/routes/login/+page.svelte`: fetch Web UI transport settings and show an insecure-login warning when HTTP login is possible or current transport is HTTP.
- `frontend/src/routes/settings/+page.svelte`: replace placeholder with a read-only Web UI transport settings panel.
- `docs/OPERATIONS.md`: document dev/test HTTP mode, env variable behavior, and HAProxy-optional posture.
- `docs/SECURITY.md`: document HTTP login warning, Secure-cookie default, and production TLS expectations.

## Task 1: Backend Web UI Settings Endpoint

**Owner:** `backend-api` subagent, then `security-rbac` or `code-review` subagent for review.

**Files:**
- Modify: `backend/internal/server/server.go`
- Modify: `backend/internal/server/server_test.go`

- [ ] **Step 1: Write failing backend endpoint tests**

Add these tests to `backend/internal/server/server_test.go` near the other API endpoint tests:

```go
func TestWebUISettingsReportsSecureDefault(t *testing.T) {
	h := newTestHarnessWithCookieSecure(t, true)

	rec := serveAPI(t, h.router, http.MethodGet, "/api/settings/webui", nil, "")

	assertStatus(t, rec, http.StatusOK)
	assertJSONField(t, rec, "cookieSecure", true)
	assertJSONField(t, rec, "httpLoginAllowed", false)
	assertJSONField(t, rec, "requestSecure", false)
	assertJSONField(t, rec, "transport", "http")
	assertBodyExcludesTerms(t, rec.Body.String(), "ocserv_panel_session", "password", "token", "hash", "PRIVATE KEY")

	var body map[string]any
	decodeJSON(t, rec, &body)
	warning, ok := body["warning"].(string)
	if !ok || !strings.Contains(warning, "not encrypted") {
		t.Fatalf("expected HTTP transport warning, got %#v", body["warning"])
	}
}

func TestWebUISettingsReportsHTTPCompatibleCookieMode(t *testing.T) {
	h := newTestHarnessWithCookieSecure(t, false)

	rec := serveAPI(t, h.router, http.MethodGet, "/api/settings/webui", nil, "")

	assertStatus(t, rec, http.StatusOK)
	assertJSONField(t, rec, "cookieSecure", false)
	assertJSONField(t, rec, "httpLoginAllowed", true)
	assertJSONField(t, rec, "requestSecure", false)
	assertJSONField(t, rec, "transport", "http")

	var body map[string]any
	decodeJSON(t, rec, &body)
	warning, ok := body["warning"].(string)
	if !ok || !strings.Contains(warning, "HTTP login sends credentials") {
		t.Fatalf("expected HTTP login warning, got %#v", body["warning"])
	}
}

func TestWebUISettingsReportsSecureRequest(t *testing.T) {
	h := newTestHarnessWithCookieSecure(t, true)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/webui", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
	assertJSONField(t, rec, "cookieSecure", true)
	assertJSONField(t, rec, "httpLoginAllowed", false)
	assertJSONField(t, rec, "requestSecure", true)
	assertJSONField(t, rec, "transport", "https")
	assertJSONField(t, rec, "warning", "")
}

func TestWebUISettingsRejectsNonGET(t *testing.T) {
	h := newTestHarness(t)

	rec := serveAPI(t, h.router, http.MethodPost, "/api/settings/webui", nil, "")

	assertStatus(t, rec, http.StatusMethodNotAllowed)
}
```

Add `crypto/tls` to the test imports. Replace the existing `newTestHarness` body with a wrapper plus a configurable helper:

```go
func newTestHarness(t *testing.T) *testHarness {
	t.Helper()
	return newTestHarnessWithCookieSecure(t, true)
}

func newTestHarnessWithCookieSecure(t *testing.T, cookieSecure bool) *testHarness {
	t.Helper()

	ctx := context.Background()
	database, err := db.OpenSQLite(ctx, filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})
	if err := db.ApplyMigrations(ctx, database, migrationsDir(t)); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	s := store.New(database)
	recorder := audit.NewSQLiteRecorder(s)
	authService := app.NewAuthService(s, recorder)
	dashboardService := app.NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())

	return &testHarness{
		database: database,
		store:    s,
		router:   NewAppRouter(authService, dashboardService, AppOptions{CookieSecure: cookieSecure}),
	}
}
```

- [ ] **Step 2: Run tests to verify RED**

Run:

```bash
cd /opt/git/ocserv_panel/backend
PATH="$HOME/.local/bin:$PATH" go test -count=1 ./internal/server
```

Expected: FAIL because `/api/settings/webui` is not registered and `newTestHarnessWithCookieSecure` is not implemented yet.

- [ ] **Step 3: Implement endpoint and helpers**

In `backend/internal/server/server.go`, add response type near the other response structs:

```go
type webUISettingsResponse struct {
	CookieSecure     bool   `json:"cookieSecure"`
	HTTPLoginAllowed bool   `json:"httpLoginAllowed"`
	RequestSecure    bool   `json:"requestSecure"`
	Transport        string `json:"transport"`
	Warning          string `json:"warning"`
}
```

Register the route in `NewAppRouter`:

```go
	mux.HandleFunc("/api/settings/webui", appHandler.handleWebUISettings)
```

Add the handler and helper functions near the other handler methods:

```go
func (a *App) handleWebUISettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	requestSecure := requestIsSecure(r)
	transport := "http"
	if requestSecure {
		transport = "https"
	}

	response := webUISettingsResponse{
		CookieSecure:     a.cookieSecure,
		HTTPLoginAllowed: !a.cookieSecure,
		RequestSecure:    requestSecure,
		Transport:        transport,
		Warning:          webUITransportWarning(!a.cookieSecure, requestSecure),
	}
	writeJSON(w, http.StatusOK, response)
}

func requestIsSecure(r *http.Request) bool {
	return r.TLS != nil
}

func webUITransportWarning(httpLoginAllowed bool, requestSecure bool) string {
	if httpLoginAllowed {
		return "HTTP login sends credentials/session cookies without transport encryption. Use only on trusted local networks or for testing."
	}
	if !requestSecure {
		return "HTTP login is not enabled, but this request is not encrypted. Use HTTPS before entering credentials on untrusted networks."
	}
	return ""
}
```

- [ ] **Step 4: Run backend verification**

Run:

```bash
cd /opt/git/ocserv_panel/backend
PATH="$HOME/.local/bin:$PATH" gofmt -w internal/server/server.go internal/server/server_test.go
PATH="$HOME/.local/bin:$PATH" go test -count=1 ./internal/server
PATH="$HOME/.local/bin:$PATH" go test -count=1 ./...
```

Expected: PASS for `./internal/server` and all backend packages.

## Task 2: Frontend API Helper and Login Warning

**Owner:** `frontend-admin-ux` subagent, then `security-rbac` or `code-review` subagent for review.

**Files:**
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/routes/login/+page.svelte`

- [ ] **Step 1: Add failing frontend references**

First update `frontend/src/routes/login/+page.svelte` to import and call `getWebUISettings` before adding it to `api.ts`. The check command should fail with a missing export.

Use this target login page structure:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
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
      const [setup, settings] = await Promise.all([getSetupStatus(), getWebUISettings()]);
      setupRequired = setup.required;
      webUISettings = settings;
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

  function webUISettingsWarning(settings: WebUISettings | null): string {
    if (!settings) return '';
    if (settings.warning) return settings.warning;
    if (settings.httpLoginAllowed || settings.transport === 'http') {
      return 'HTTP login sends credentials/session cookies without transport encryption. Use only on trusted local networks or for testing.';
    }
    return '';
  }
</script>

<section class="auth-card panel">
  <p class="eyebrow">Login</p>
  <h2>Admin sign in</h2>
  <p class="muted">The backend sets an HttpOnly session cookie. This page never stores tokens or passwords.</p>

  {#if transportWarning}
    <div class="callout warning">
      <strong>Insecure login warning</strong>
      <p>{transportWarning}</p>
    </div>
  {/if}

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
```

- [ ] **Step 2: Run check to verify RED**

Run:

```bash
cd /opt/git/ocserv_panel/frontend
PATH="$HOME/.local/bin:$PATH" npm run check
```

Expected: FAIL because `$lib/api` does not export `getWebUISettings` or `WebUISettings`.

- [ ] **Step 3: Implement API helper**

Add this type and helper to `frontend/src/lib/api.ts` after `AuthSummary`:

```ts
export type WebUISettings = {
  cookieSecure: boolean;
  httpLoginAllowed: boolean;
  requestSecure: boolean;
  transport: 'http' | 'https';
  warning: string;
};
```

Add this function near the other API helpers:

```ts
export function getWebUISettings(): Promise<WebUISettings> {
  return apiFetch<WebUISettings>('/api/settings/webui');
}
```

- [ ] **Step 4: Run frontend verification**

Run:

```bash
cd /opt/git/ocserv_panel/frontend
PATH="$HOME/.local/bin:$PATH" npm run check
PATH="$HOME/.local/bin:$PATH" npm run build
```

Expected: `npm run check` reports `svelte-check found 0 errors and 0 warnings`; `npm run build` completes, with the existing `adapter-auto` advisory accepted.

## Task 3: Settings Web UI Transport Panel

**Owner:** `frontend-admin-ux` subagent, then `security-rbac` or `code-review` subagent for review.

**Files:**
- Modify: `frontend/src/routes/settings/+page.svelte`

- [ ] **Step 1: Replace placeholder with read-only settings page**

Replace `frontend/src/routes/settings/+page.svelte` with:

```svelte
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
```

- [ ] **Step 2: Run frontend verification**

Run:

```bash
cd /opt/git/ocserv_panel/frontend
PATH="$HOME/.local/bin:$PATH" npm run check
PATH="$HOME/.local/bin:$PATH" npm run build
```

Expected: check and build pass; the existing `adapter-auto` advisory remains acceptable.

## Task 4: Documentation Updates

**Owner:** `docs-adr` subagent, then `deployment-ops` or `security-rbac` review.

**Files:**
- Modify: `docs/OPERATIONS.md`
- Modify: `docs/SECURITY.md`

- [ ] **Step 1: Update operations documentation**

Add or update a section in `docs/OPERATIONS.md` with this content:

```markdown
## HTTP login mode for local or trusted deployments

HAProxy is optional for local testing and trusted single-host use. The backend can allow login over plain HTTP when started with:

```bash
OCSERV_PANEL_COOKIE_SECURE=false
```

This makes the session cookie HTTP-compatible while keeping it `HttpOnly` and `SameSite=Lax`. The Settings tab displays the current mode but does not change it. Restart the backend after changing `OCSERV_PANEL_COOKIE_SECURE`.

For exposed production deployments, use a TLS strategy before entering credentials. The supported strategy for this baseline is HAProxy TLS termination in front of the loopback Go backend. Another reverse proxy or future direct Go TLS support can be documented later.
```
```

- [ ] **Step 2: Update security documentation**

Add or update a section in `docs/SECURITY.md` with this content:

```markdown
## HTTP login warning

HTTP-compatible login is supported only when `OCSERV_PANEL_COOKIE_SECURE=false`. In that mode, credentials and session cookies are not protected by transport encryption. Use this only for local testing or trusted networks.

The login page and Settings tab warn when HTTP login is possible or when the current request is plain HTTP. These warnings do not make HTTP secure; they only make the risk visible.

Session cookies remain `HttpOnly` and `SameSite=Lax` in both modes. SQLite stores only password hashes and session token hashes, never plaintext passwords or raw session tokens.
```
```

- [ ] **Step 3: Review docs for contradictions**

Run:

```bash
cd /opt/git/ocserv_panel
grep -R "HAProxy is required\|HTTP login is not supported\|COOKIE_SECURE=false.*not" docs || true
```

Expected: no contradictory statement remains. If a matching line is a valid warning rather than a contradiction, keep it and note it in the handoff.

## Task 5: Security Review and Full Verification

**Owner:** `security-rbac`, `test-verification`, then final `code-review` subagents.

**Files:**
- Review: `backend/internal/server/server.go`
- Review: `backend/internal/server/server_test.go`
- Review: `frontend/src/lib/api.ts`
- Review: `frontend/src/routes/login/+page.svelte`
- Review: `frontend/src/routes/settings/+page.svelte`
- Review: `docs/OPERATIONS.md`
- Review: `docs/SECURITY.md`

- [ ] **Step 1: Security review checklist**

Confirm all of these statements are true:

```text
- /api/settings/webui is read-only and does not require authentication.
- /api/settings/webui exposes only booleans/strings about transport posture.
- /api/settings/webui does not expose cookie values, raw tokens, password hashes, private keys, filesystem paths, certificate paths, or env dumps.
- HTTP-compatible login remains controlled only by OCSERV_PANEL_COOKIE_SECURE=false.
- Existing session cookies remain HttpOnly and SameSite=Lax.
- No browser storage or manual cookie handling was added in frontend code.
- No live ocserv, systemd, sudo, HAProxy, certificate, service-control, or config mutation code was added.
```

- [ ] **Step 2: Full verification commands**

Run:

```bash
cd /opt/git/ocserv_panel/backend
PATH="$HOME/.local/bin:$PATH" go test -count=1 ./...

cd /opt/git/ocserv_panel/frontend
PATH="$HOME/.local/bin:$PATH" npm run check
PATH="$HOME/.local/bin:$PATH" npm run build

cd /opt/git/ocserv_panel
grep -R "localStorage\|sessionStorage\|document.cookie\|child_process\|node:fs\|occtl\|ocpasswd\|systemctl\|sudo\|PRIVATE KEY" frontend/src backend/internal/server docs/OPERATIONS.md docs/SECURITY.md || true
```

Expected:

- Go tests pass for all backend packages.
- Svelte check reports `0 errors and 0 warnings`.
- SvelteKit build completes; the existing `adapter-auto` advisory is acceptable.
- Safety search has no frontend/backend live-command matches. Documentation may mention `sudo`, HAProxy, or HTTP warnings only as operational/security text.

- [ ] **Step 3: Final status handoff**

Return:

```text
Status: DONE or DONE_WITH_CONCERNS
Files changed:
- ...
Verification:
- go test -count=1 ./...: ...
- npm run check: ...
- npm run build: ...
Security notes:
- ...
Known limitations:
- Web UI displays but does not mutate the transport setting.
- Direct TLS support remains out of scope.
```

## Execution Handoff

Plan complete. Execute with subagent-driven development. Use one implementation subagent per task and review after each task. Do not commit unless the user explicitly requests a commit.
