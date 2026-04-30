# Deployment Assets

This directory is reserved for local deployment assets for the `ocserv` admin panel.

The intended runtime model is:

- The Go backend runs as a local systemd service on the same host as `ocserv`.
- HAProxy terminates TLS and forwards requests to the backend loopback listener.
- Future privileged operations use narrow, documented permission boundaries.
- SQLite remains local to the host.

No files in this baseline are installed automatically. Do not run `systemctl`, `sudo`, `ocserv`, `occtl`, or `ocpasswd` as part of baseline verification.

Future deployment work should add reviewed example assets for:

- `ocserv-panel.service`
- HAProxy frontend/backend snippets
- Minimal sudoers rules, if privileged operations require them
- Filesystem ownership and permission guidance

Any future service-control or configuration mutation path must validate inputs, create backups where applicable, and produce audit records.
