# Operations Guide

## Runtime Model

`ocserv_panel` is designed to run on the same host as `ocserv`.

Expected components:

- Go backend listening on a loopback address such as `127.0.0.1:8080`.
- HAProxy terminating TLS and forwarding to the backend.
- SQLite database stored on the local filesystem.
- systemd managing the backend process in future deployment work.

The API is same-origin under `/api`. In production, HAProxy terminates TLS and forwards to the loopback backend. In local development, the SvelteKit/Vite dev server may proxy `/api` to `127.0.0.1:8080`; this does not require CORS.

## Remote Access via SSH Port Forwarding

When the panel is deployed on a remote server, you can access the Web UI securely over an SSH tunnel without exposing the port to the internet.

### Production deployment (HTTPS on port 8443)

The default systemd unit listens on `0.0.0.0:8443` with TLS enabled. Forward the remote port to your local machine:

```sh
ssh -L 8443:127.0.0.1:8443 user@your-server
```

Then open **https://localhost:8443** in your browser. Because the TLS certificate is issued for the server's domain (not `localhost`), your browser will show a certificate warning — this is safe to accept for local testing since the traffic never leaves the SSH tunnel.

To avoid the certificate warning, add the server's domain to your local `/etc/hosts`:

```sh
# Add to /etc/hosts (replace with your actual domain)
127.0.0.1  light-mirror.ru
```

Then open **https://light-mirror.ru:8443** — the certificate will match and the browser will not warn.

### Development deployment (HTTP on port 8080)

If running in HTTP mode (e.g., `OCSERV_PANEL_ADDR=127.0.0.1:8080` without TLS certs):

```sh
ssh -L 8080:127.0.0.1:8080 user@your-server
```

Then open **http://localhost:8080** in your browser.

### Persistent tunnel (background mode)

To keep the tunnel running in the background:

```sh
ssh -f -N -L 8443:127.0.0.1:8443 user@your-server
```

Flags:
- `-f` — go to background after authentication
- `-N` — do not execute a remote command (tunnel only)
- `-L` — local port forwarding

### Using an SSH config shortcut

Add to `~/.ssh/config` for convenience:

```
Host ocserv-panel
    HostName your-server-ip
    User your-user
    LocalForward 8443 127.0.0.1:8443
```

Then simply run:

```sh
ssh -N ocserv-panel
```

### Adding a tunnel from an existing SSH session

If you are already connected to the server via SSH, open a **new local terminal** and run:

```sh
ssh -f -N -L 8443:127.0.0.1:8443 user@your-server
```

This creates a background tunnel alongside your existing session. Then open **https://localhost:8443** on your local machine.

Alternatively, start a second SSH connection with the tunnel:

```sh
ssh -L 8443:127.0.0.1:8443 user@your-server
```

Multiple SSH sessions to the same host work simultaneously without conflict.

### Notes

- The SSH tunnel encrypts all traffic between your machine and the server, so even the HTTP mode is secure in transit.
- Only one SSH session needs to be active for the tunnel to work; you can use it alongside a normal shell session.
- If port 8443 is already in use locally, pick a different local port: `ssh -L 9443:127.0.0.1:8443 user@your-server` and open `https://localhost:9443`.

---

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

## HTTP login mode for local or trusted deployments

HAProxy is optional for local testing and trusted single-host use. The backend can allow login over plain HTTP when started with:

```bash
OCSERV_PANEL_COOKIE_SECURE=false
```

This makes the session cookie HTTP-compatible while keeping it `HttpOnly` and `SameSite=Lax`. The Settings tab displays the current mode but does not change it. Restart the backend after changing `OCSERV_PANEL_COOKIE_SECURE`.

For exposed production deployments, use a TLS strategy before entering credentials. The supported strategy for this baseline is HAProxy TLS termination in front of the loopback Go backend. Another reverse proxy or future direct Go TLS support can be documented later.

## Dashboard Analytics and Purge

The current dashboard/history data is local-only. Historical sample session rows may be seeded as sample data for development and initial UI verification.

### Local Monitoring TSDB

Dashboard monitoring uses the embedded Prometheus TSDB Go library as a local, file-backed time-series store inside the backend process. This is not a Prometheus server, daemon, scrape target, Grafana deployment, or external monitoring stack.

Operational defaults and configuration:

- `OCSERV_PANEL_TSDB_PATH` controls the TSDB directory and defaults to `data/metrics`.
- Retention is 30 days.
- Collectors run every 15 seconds.
- Host CPU and memory collectors run by default.
- Network and VPN interface counters are collected only when `OCSERV_PANEL_NET_IFACE` and/or `OCSERV_PANEL_VPN_IFACE` name local interfaces.
- `ocserv`, user, and session metric slots may appear as unavailable until future ingestion is implemented; unavailable values should not be interpreted as zero activity.

If the TSDB cannot be opened, the backend falls back to in-memory metrics so dashboard reads can continue. Treat this as operationally degraded: new samples are not persistent and will be lost on restart until the TSDB path or filesystem permissions are fixed.

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
