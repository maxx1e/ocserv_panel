# Operations Guide

## Runtime Model

`ocserv_panel` is designed to run on the same host as `ocserv`.

Expected components:

- Go backend listening on a loopback address such as `127.0.0.1:8080`.
- HAProxy terminating TLS and forwarding to the backend.
- SQLite database stored on the local filesystem.
- systemd managing the backend process in future deployment work.

The API is same-origin under `/api`. In production, HAProxy terminates TLS and forwards to the loopback backend. In local development, the SvelteKit/Vite dev server may proxy `/api` to `127.0.0.1:8080`; this does not require CORS.

## Local Development Without `ocserv`

The authenticated dashboard slice can run without a live `ocserv` instance. It provides first-run owner setup, login/logout, authenticated dashboard summary/history APIs, and local sample analytics.

Example sequence:

1. Start the backend with a local SQLite DSN and migrations directory:

   ```sh
   OCSERV_PANEL_DB_DSN=./ocserv-panel.dev.db \
   OCSERV_PANEL_MIGRATIONS_DIR=./migrations \
   OCSERV_PANEL_COOKIE_SECURE=false \
   go run ./backend/cmd/ocserv-panel
   ```

   Use `OCSERV_PANEL_COOKIE_SECURE=false` only for loopback HTTP development. The production default is a Secure session cookie.

2. Start the frontend development server from the frontend directory:

   ```sh
   npm run dev
   ```

3. Open the SvelteKit dev URL, complete first-run owner setup, then sign in through the login page.

The frontend does not read cookies directly, use browser storage for authentication, or access local files, sockets, commands, certificates, or secrets.

## Authentication Cookie Expectations

Session cookies are HttpOnly and SameSite=Lax. The backend default should keep the cookie `Secure` for production behind HAProxy TLS. Only explicit local development configuration should disable `Secure`, and only when using loopback HTTP.

## Dashboard Analytics and Purge

The current dashboard/history data is local-only. Historical sample session rows may be seeded as sample data for development and initial UI verification.

The history purge operation deletes only historical session rows older than the configured retention window. It is owner-only and requires the confirmation string `PURGE`. It does not terminate active VPN sessions, call `ocserv`, edit configuration, or remove certificates.

## Current Slice Safety

The authenticated dashboard slice does not install services, modify live `ocserv` configuration, access the `ocserv` management socket, issue certificates, read private keys, or run `ocserv`, `occtl`, `ocpasswd`, `systemctl`, `sudo`, or other privileged service-control commands. Reported `ocserv` status may be `not_configured` until live integration is implemented.

Do not assume root access is available. Future operations that require elevated permissions must document:

- Required runtime user and groups.
- Exact filesystem paths and permissions.
- Minimal sudoers rules, if any.
- Validation and backup steps before configuration changes.
- Audit events emitted for administrative mutations.

## Future Safe Apply Expectations

Configuration editing must follow this sequence:

1. Parse and validate the proposed configuration.
2. Show a preview or diff to the administrator.
3. Create a backup of the current configuration.
4. Apply through a narrow backend interface.
5. Verify the result and report errors clearly.
6. Preserve an audit record for success, failure, or denial.

Normal tests should use fakes for service control, filesystem writes, and `ocserv` integration.
