# HTTP Login Settings Visibility Design

## Objective

Expose the Web UI transport/login mode in the Settings tab and warn users when HTTP-compatible login is enabled or when the panel is accessed over plain HTTP. HAProxy remains optional. The source of truth for allowing HTTP login in this slice is still the backend environment variable `OCSERV_PANEL_COOKIE_SECURE`.

## Decisions

- Keep the current `owner` and `viewer` roles. Do not add an `admin` role in this slice.
- Keep `OCSERV_PANEL_COOKIE_SECURE` as the only control for session cookie `Secure` behavior.
- Keep secure cookies enabled by default when the environment variable is unset or true-like.
- Allow HTTP login when `OCSERV_PANEL_COOKIE_SECURE=false` by issuing non-`Secure`, `HttpOnly`, `SameSite=Lax` session cookies.
- Add read-only Web UI visibility for this mode. Do not add a Web UI mutation path for the setting.
- Show clear warnings on the login page and Settings page when HTTP login is possible or the current request is plain HTTP.
- Do not introduce direct TLS management, certificate path editing, HAProxy mutation, systemd mutation, or live ocserv operations.

## Backend Design

Add a read-only endpoint:

```text
GET /api/settings/webui
```

The endpoint returns JSON describing the current Web UI transport posture:

```json
{
  "cookieSecure": true,
  "httpLoginAllowed": false,
  "requestSecure": false,
  "transport": "http",
  "warning": "HTTP login is not enabled, but this request is not encrypted. Use HTTPS before entering credentials on untrusted networks."
}
```

Field meanings:

- `cookieSecure`: backend session cookies are configured with the `Secure` attribute.
- `httpLoginAllowed`: login over HTTP can work because cookies are not configured `Secure`.
- `requestSecure`: the current API request reached the backend as HTTPS according to direct request state for this slice.
- `transport`: `https` when `requestSecure` is true, otherwise `http`.
- `warning`: empty string when there is no transport warning; otherwise a user-facing warning.

Security behavior:

- This endpoint is read-only and does not require authentication, because the login page needs it before a session exists.
- It must not expose secrets, cookie values, session tokens, password hashes, filesystem paths, certificate paths, or environment dumps.
- It must not mutate SQLite settings, live services, HAProxy, certificates, ocserv, or systemd.
- Existing login cookie behavior remains unchanged except for surfacing its configured mode through this endpoint.

Request security detection for this slice:

- Treat `r.TLS != nil` as secure.
- Do not trust arbitrary forwarded headers in this slice, because HAProxy trust-boundary handling has not been implemented yet.
- Documentation can note that future production proxy support may add explicit trusted-proxy header handling.

## Frontend Design

### Settings Tab

Replace the Settings placeholder with a read-only “Web UI transport” panel that calls `GET /api/settings/webui` on mount.

It displays:

- Cookie mode: `Secure cookies` or `HTTP-compatible cookies`.
- Current request transport: `HTTPS` or `HTTP`.
- Whether HTTP login is currently possible.
- Exact environment guidance:
  - set `OCSERV_PANEL_COOKIE_SECURE=false` to allow HTTP login;
  - unset it or set a true-like value to use HTTPS-oriented secure cookies.

The Settings tab does not submit changes in this slice.

### Login Page Warning

The login page calls `GET /api/settings/webui` on mount. It shows a prominent warning when either condition is true:

- `httpLoginAllowed` is true; or
- `transport` is `http`.

Warning text must clearly state:

```text
HTTP login sends credentials/session cookies without transport encryption. Use only on trusted local networks or for testing.
```

The login page must not read cookies directly, use browser storage for auth, or display raw tokens/passwords/hashes.

## Documentation Design

Update operational and security docs to state:

- HAProxy is optional for local/test usage.
- HTTP login is supported when `OCSERV_PANEL_COOKIE_SECURE=false`.
- HTTP login is not secure on untrusted networks because credentials and session cookies are not protected by transport encryption.
- Production deployments exposed beyond trusted local environments should use a TLS strategy such as HAProxy TLS termination, another reverse proxy, or future direct TLS support.
- This slice does not manage TLS certificates, HAProxy config, systemd units, or live service state.

## Testing and Verification

Backend tests:

- `GET /api/settings/webui` reports secure-cookie mode by default.
- `GET /api/settings/webui` reports HTTP-compatible mode when `CookieSecure` is false.
- `GET /api/settings/webui` returns no secret-bearing fields.
- Existing login cookie tests continue to prove the `Secure` flag follows configuration.

Frontend checks:

- Settings page compiles and renders the read-only transport status paths.
- Login page compiles and includes the HTTP warning path.
- No browser storage, manual cookie handling, local filesystem, command, certificate, or secret access is introduced.

Verification commands:

```bash
cd backend && PATH="$HOME/.local/bin:$PATH" go test -count=1 ./...
cd frontend && PATH="$HOME/.local/bin:$PATH" npm run check
cd frontend && PATH="$HOME/.local/bin:$PATH" npm run build
```

## Out of Scope

- Web UI mutation of the transport setting.
- Adding an `admin` role.
- Direct TLS listener support in the Go backend.
- HAProxy configuration generation or mutation.
- Certificate upload, issuance, revocation, or filesystem management.
- Any live ocserv, systemd, sudo, or service-control operation.
