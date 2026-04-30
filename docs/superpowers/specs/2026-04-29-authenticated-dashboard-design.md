# Authenticated Dashboard Slice Design

## Objective

Implement the first real Web UI panel slice for `ocserv_panel`: a local-first authenticated admin dashboard with first-run owner setup, login/logout session handling, read-only operational status, sample-backed historical analytics, and an owner-only historical analytics purge action.

This slice proves the full SvelteKit frontend -> Go backend `/api` -> SQLite flow while preserving the project boundary that all privileged ocserv/system operations remain backend-owned and inactive until later, explicitly reviewed work.

## Decisions

- Keep the custom Go backend architecture. Do not pivot to PocketBase.
- Do not introduce a separate PocketBase service or other external service.
- Use `modernc.org/sqlite` as the backend SQLite driver so the local database works without CGO or system SQLite compiler tooling.
- Use same-origin `/api/...` calls. In development, Vite proxies `/api` to the Go backend. In production, HAProxy will route same-origin traffic to the backend.
- Include real login/session authentication now, rather than dev-only headers.
- Use a first-run Web setup page to create the initial `owner` admin when no admin exists.
- Avoid external Go password-hashing dependencies. Use Go standard-library PBKDF2-HMAC-SHA256 with a random per-user salt for admin password hashes.
- Set the backend minimum Go version to Go 1.24 or newer so `crypto/pbkdf2` is available from the standard library. The current development environment has Go 1.26.
- Store only password hashes, session token hashes, safe metadata, audit events, and analytics records in SQLite.
- Seed sample historical analytics data and label it as sample/demo data in API responses and UI.
- Add an owner-only UI and backend API to purge historical analytics records older than a selected cutoff.
- Do not add live ocserv integration, live session termination, user lifecycle changes, config mutation, certificate mutation, sudo, systemd service control, or filesystem writes beyond SQLite database access.

## Explicit Non-Goals

- No VPN user creation, deletion, password change, or certificate issuance/revocation.
- No live `ocserv`, `occtl`, `ocpasswd`, management socket, `systemctl`, or `sudo` execution.
- No live ocserv config parsing, editing, backup, apply, or rollback.
- No Prometheus, Grafana, queues, cloud control plane, hosted service, or distributed state.
- No PocketBase dependency, service, or framework integration in this slice.
- No external Go dependency except the approved pure-Go SQLite driver `modernc.org/sqlite` and its transitive build/runtime dependencies.
- No frontend access to local files, sockets, commands, certificates, or secrets.

## Backend Architecture

The Go backend remains the only API and privileged-boundary owner. It will add an application dependency set that wires:

- SQLite repositories for admin accounts, admin sessions, audit events, and historical analytics.
- Password hashing and verification using standard-library PBKDF2-HMAC-SHA256.
- Session token generation, hashing, cookie issuance, cookie clearing, and session revocation.
- Authentication middleware that resolves the current admin from an `HttpOnly` session cookie.
- RBAC middleware that authorizes owner/viewer roles for read APIs and owner-only purge.
- Dashboard/history handlers that use the existing no-op ocserv manager for safe `not_configured` operational state.

The server should keep small, explicit interfaces for repositories and service adapters so future ocserv integration can remain fake-testable.

## Authentication and Sessions

First-run setup flow:

1. `GET /api/setup/status` returns whether setup is required.
2. If no admin exists, `POST /api/setup/owner` accepts username and password.
3. The backend validates the username and password length, hashes the password, creates the owner role/account inside a transaction, writes an audit event, and refuses if any admin already exists.

Login/session flow:

1. `POST /api/auth/login` validates credentials with a generic failure response.
2. On success, the backend creates a random opaque session token, stores only a token hash in SQLite, and sets an `HttpOnly`, `SameSite=Lax` cookie.
3. `GET /api/auth/me` returns the current admin identity and roles when a valid session exists.
4. `POST /api/auth/logout` revokes the current session, clears the cookie, and audit-logs the action.

Cookie security defaults:

- `HttpOnly` is always set.
- `SameSite=Lax` is always set.
- `Path=/` is used for the panel.
- `Secure` is enabled by configuration for production behind HAProxy TLS; development may use non-secure cookies only on loopback.
- Sessions expire after 12 hours. Logout revokes the current session before clearing the cookie.

No plaintext passwords or plaintext session tokens are stored in SQLite.

## Password Hash Format

The encoded password hash will include the algorithm and parameters:

```text
pbkdf2-sha256$<iterations>$<base64-salt>$<base64-derived-key>
```

New hashes use 600,000 iterations, a 16-byte random salt from `crypto/rand`, and a 32-byte derived key. Verification parses the encoded hash, derives a key from the submitted password using the stored parameters, and compares the derived key in constant time.

## SQLite Persistence

The schema will extend the baseline migration model with deterministic migrations for:

- `admin_users`: username, password hash, enabled flag, timestamps.
- `admin_user_roles`: admin-to-role mapping for assigning `owner` and future `viewer` roles.
- `admin_sessions`: admin ID, token hash, expiry, revoked timestamp, timestamps.
- `audit_log`: persisted audit events for setup, login, logout, API denials, and purge.
- `historical_sessions`: sample/demo analytics records with source marker and timestamps.

The seeded historical records must be visibly marked as sample/demo data. Future ingestion can replace or append real records without confusing sample records for operational truth.

Sample data is inserted only when the historical analytics table is empty and every seeded record has source `sample`. The purge action deletes only historical analytics/session records older than the selected cutoff. It must not delete admin users, admin sessions, audit log records, config metadata, certificate metadata, or app settings.

## API Design

All API endpoints are same-origin `/api/...` routes.

- `GET /api/health`: backend/API health.
- `GET /api/setup/status`: reports whether first-run setup is required.
- `POST /api/setup/owner`: creates the first owner account only when no admin exists.
- `POST /api/auth/login`: creates a session cookie on valid credentials.
- `POST /api/auth/logout`: revokes the current session and clears the cookie.
- `GET /api/auth/me`: returns current admin identity and roles.
- `GET /api/dashboard/summary`: authenticated read endpoint with backend health, ocserv adapter status `not_configured`, active session count from the no-op manager, recent audit events, and analytics summary.
- `GET /api/history/analytics`: authenticated read endpoint returning sample-backed historical analytics from SQLite with sample/demo labeling.
- `POST /api/history/purge`: owner-only destructive endpoint that requires confirmation text `PURGE`, validates a retention option of 30, 90, 180, or 365 days, deletes eligible historical analytics records older than the computed cutoff, and audit-logs success, failure, and denial.

HTTP handlers should return consistent JSON error responses without leaking whether a username exists during login. Unauthorized requests return `401`; authenticated-but-forbidden requests return `403`.

## Frontend UX

The SvelteKit app will use only backend APIs and same-origin fetches.

States:

- If setup is required, show a first-run setup page for the initial owner account.
- If setup is complete and no session is valid, show login.
- If logged in, show the dashboard.

Authenticated dashboard content:

- Current admin username and roles.
- Backend/API health.
- ocserv adapter state, clearly shown as `not_configured`.
- Active session count from the safe no-op manager.
- Recent audit events.
- Historical analytics summary with sample/demo labeling.

History page content:

- Sample historical analytics records and aggregate cards.
- Clear text explaining that sample data will be replaced by future real session ingestion.
- Owner-only purge panel with retention selection of 30, 90, 180, or 365 days and destructive confirmation text `PURGE`.
- Viewer users can read dashboard/history but cannot see or submit the purge action.

Errors should be operationally clear without exposing internal stack traces or sensitive details.

## RBAC and Audit Requirements

Role behavior:

- `owner`: can read dashboard/history and purge historical analytics.
- `viewer`: can read dashboard/history only.
- unknown or missing roles: deny by default.

Audit behavior:

- First owner setup success/failure.
- Login success/failure.
- Logout success/failure.
- Unauthorized or forbidden access attempts for protected dashboard, history, and purge APIs.
- Purge success/failure/denial, including cutoff and number of deleted historical records as safe metadata.

Audit records must not contain plaintext passwords, session tokens, private keys, or secrets.

## Security Considerations

- This slice crosses authentication, session management, RBAC, audit logging, persistence, and destructive purge boundaries, so it requires security-focused tests and final review.
- Use `crypto/rand` for 32-byte session tokens and 16-byte password salts.
- Compare password-derived keys and session token hashes using constant-time comparison where applicable.
- Store session token hashes, not raw tokens.
- Avoid detailed login failure messages.
- Do not add CORS for this slice; same-origin `/api` avoids cross-origin credential complexity.
- Do not expose privileged system/ocserv actions through frontend or backend endpoints in this slice.

## Testing and Verification

Backend tests:

- Password hashes encode parameters, do not contain plaintext passwords, use random salts, and verify correct/incorrect passwords.
- First-run setup succeeds once and refuses after an admin exists.
- Login succeeds with valid credentials and fails generically with invalid credentials.
- Logout revokes only the current session.
- Protected dashboard/history endpoints require a valid session.
- RBAC denies missing/unknown roles and denies viewer purge.
- Owner purge requires confirmation text, validates cutoff, deletes only historical analytics records, and audit-logs outcomes.
- Repositories use parameterized SQL and context-aware database operations.

Frontend checks:

- `npm run check` passes.
- Setup/login/dashboard/history states render correctly from API responses.
- Purge UI requires owner role, cutoff selection, and confirmation text.

Repository verification:

- `go test ./...` from `backend/`.
- `npm run check` and `npm run build` from `frontend/`.
- Read-only safety search confirms no live `ocserv`, `occtl`, `ocpasswd`, `systemctl`, `sudo`, `os/exec`, `child_process`, or frontend filesystem access was introduced.

## Subagent Development Plan

Implementation will use focused subagents and avoid parallel edits to the same migration, API contract, shared auth type, or privileged operation:

1. SQLite persistence agent for migrations and repositories.
2. Backend API agent for auth/session/setup/dashboard/history handlers.
3. Security/RBAC agent for middleware, deny-path tests, session cookie review, and purge authorization review.
4. Frontend admin UX agent for setup/login/dashboard/history/purge UI.
5. Documentation/Ops agent for updated operational and security docs.
6. Test/verification agent for full backend/frontend verification.
7. Code review agent for final security and architecture review.

## Acceptance Criteria

- A new local admin can be created through first-run setup when no admin exists.
- Setup is refused after an admin exists.
- Admin can log in, access the dashboard/history pages, and log out.
- Dashboard and history APIs require authentication.
- Historical analytics are sample-backed and labeled as sample/demo data.
- Owner can purge old historical analytics with confirmation; viewer cannot.
- Purge deletes only historical analytics records and is audit-logged.
- No plaintext passwords, raw session tokens, private keys, tokens, or credentials are stored in SQLite or source control.
- No live ocserv/system/service/config/certificate operations are introduced.
- `go test ./...`, `npm run check`, and `npm run build` pass before completion claims.
