# Security Guide

## Security-Sensitive Boundaries

The following areas are security-sensitive:

- Authentication and session management.
- RBAC checks.
- User creation, deletion, and password changes.
- Certificate issuance, visibility, and revocation.
- Active session termination.
- `ocserv` configuration edits.
- systemd, sudo, filesystem, and management socket access.

## Required Controls

Every privileged backend action should include:

- Authentication.
- Authorization through RBAC.
- Input validation.
- Clear error handling.
- Audit logging.
- Tests using fakes or adapters where practical.

## Secret Handling

Do not store plaintext passwords, private keys, API tokens, credentials, or generated secrets in source control or SQLite.

Password-related persistence must store only password hashes. Certificate persistence must store safe metadata and must not store private keys.

## Authentication and Password Storage

First-run setup creates the owner account through `/api/setup/owner` when `/api/setup/status` reports setup is required.

Passwords are hashed with PBKDF2-HMAC-SHA256 using Go standard library cryptography:

- 600,000 iterations.
- Random 16-byte salt.
- 32-byte derived key.
- Stored format: `pbkdf2-sha256$...`.

Plaintext passwords are never stored in SQLite or source control.

## Session Tokens

Session tokens are opaque random values. The raw token is used only in client transport via an HttpOnly SameSite=Lax cookie; frontend code must not read or store it.

SQLite stores only the SHA-256 hash of the token. Logout should revoke the stored session before clearing the cookie when the request still carries a valid token.

Production deployments should keep the session cookie `Secure` behind HAProxy TLS. Disabling `Secure` is an explicit loopback-development opt-out only.

## HTTP login warning

HTTP-compatible login is supported only when `OCSERV_PANEL_COOKIE_SECURE=false`. In that mode, credentials and session cookies are not protected by transport encryption. Use this only for local testing or trusted networks.

The login page and Settings tab warn when HTTP login is possible or when the current request is plain HTTP. These warnings do not make HTTP secure; they only make the risk visible.

Session cookies remain `HttpOnly` and `SameSite=Lax` in both modes. SQLite stores only password hashes and session token hashes, never plaintext passwords or raw session tokens.

## RBAC and Audit Coverage

The backend is the source of truth for authorization. Current roles are:

- `owner`: full access for the authenticated dashboard slice, including owner-only history purge.
- `viewer`: read-oriented dashboard access only; destructive or administrative actions must be denied.

The history purge path is owner-only and requires the exact confirmation string `PURGE`.

Audit logging should cover authentication events, purge attempts and results, and denied dashboard/history access. Audit metadata must redact sensitive keys and private-key markers before persistence or display.

## Monitoring Data Safety

The monitoring TSDB must store only numeric samples and safe labels. It must not store secrets, private keys, passwords, tokens, certificates, plaintext credentials, or raw command output.

Monitoring read APIs require authenticated dashboard read access through RBAC. Successful reads, auth/RBAC denials, and backend query failures are audited; request validation failures may be rejected without a monitoring audit event.

Current collectors read local `/proc` and `/sys` files only. This monitoring slice does not run shell commands, `sudo`, `systemctl`, `occtl`, `ocpasswd`, or access the live `ocserv` management socket.

## Frontend Boundary

The SvelteKit frontend must not access local files, sockets, commands, certificates, or secrets directly. It must call backend APIs for all administrative behavior.

Authentication state is API-driven. The frontend must not use local storage, session storage, or direct cookie handling for auth tokens.

## Current Slice Posture

The authenticated dashboard slice exposes setup, login/logout/me, dashboard summary/history, and owner-only historical purge behavior. It does not execute `ocserv`, `occtl`, `ocpasswd`, `systemctl`, `sudo`, mutate live configuration, access certificate private keys, or perform destructive filesystem commands.
