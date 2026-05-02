# Initial Repository Baseline Design

## Objective

Create the initial repository baseline for a local-first, single-node `ocserv` Web UI admin panel. The baseline establishes the expected project shape, runnable backend skeleton, hand-authored frontend skeleton, initial SQLite migration placeholders, deployment documentation placeholders, and architecture/security documentation without performing privileged or destructive operations.

## Project Constraints

- Preserve the architecture from `AGENTS.md`: Go backend, SvelteKit frontend, SQLite persistence, systemd runtime management, and HAProxy TLS termination.
- Keep the product single-node and local-first for a personal-use `ocserv` server running on the same host.
- Do not introduce external services, queues, hosted dependencies, observability stacks, distributed state, cloud control planes, or container-first deployment.
- Do not execute `ocserv`, `occtl`, `ocpasswd`, `systemctl`, `sudo`, or destructive filesystem commands.
- Do not store plaintext passwords, private keys, tokens, credentials, or generated secrets in source control or SQLite.
- Use interfaces and fakes/placeholders for privileged operations.
- Keep changes small, reviewable, and separated by backend, frontend, persistence, deployment, docs, and security boundaries.

## Repository Structure

The baseline will create and use this top-level shape:

```text
backend/        Go backend API and privileged-operation boundaries
frontend/       Hand-authored SvelteKit skeleton, no dependency install
migrations/     Initial SQLite schema migration placeholders
deploy/         Deployment notes/placeholders for systemd and HAProxy
docs/           Architecture, operations, and security documentation
```

This structure follows `AGENTS.md` and keeps each ownership boundary explicit.

## Backend Design

The backend will be a small Go module under `backend/`.

Planned components:

- `cmd/ocserv-panel/main.go` starts the HTTP server.
- `internal/server` owns routing and exposes `GET /healthz`.
- `internal/auth` contains authentication placeholders and types only.
- `internal/rbac` contains role/permission placeholders and authorization helper types only.
- `internal/audit` contains an audit event model and recorder interface placeholder.
- `internal/db` contains a SQLite connection helper placeholder without storing secrets.
- `internal/ocserv` contains integration interfaces and fake/noop implementations only.

The initial server will be safe to run locally. It will not invoke `ocserv`, `occtl`, `ocpasswd`, `systemctl`, `sudo`, or mutate live configuration. Future privileged operations must be isolated behind explicit interfaces, authorized by RBAC, validated, audited, and tested with fakes.

Initial backend verification will use `go test ./...` from `backend/`. Tests should cover the health endpoint and safe placeholder behavior.

## Frontend Design

The frontend will be a manually authored minimal SvelteKit skeleton under `frontend/`. No SvelteKit scaffold command or dependency installation will run during this baseline task.

Planned files:

- `package.json` with scripts such as `dev`, `build`, and `check`.
- `svelte.config.js`, `vite.config.ts`, and `tsconfig.json`.
- `src/app.html`.
- `src/routes/+layout.svelte` with a basic shell and navigation.
- `src/routes/+page.svelte` dashboard placeholder.
- `src/routes/login/+page.svelte` login placeholder.
- Placeholder pages for users, sessions, history, config, certificates, audit, and settings.

The frontend will not perform real authentication, call privileged operations, access local files, inspect sockets, execute commands, read certificates, or handle secrets directly. It will present navigation placeholders for future backend-backed workflows.

Frontend checks will depend on local dependency availability. If dependencies are not installed, package-manager build/check commands will be skipped and reported as intentionally not run because dependency installation was avoided.

## SQLite Migration Design

Initial migration placeholders will live in `migrations/` as a deterministic SQL migration, starting with `0001_initial.sql`.

The migration will define initial tables for:

- `roles`
- `admin_users`
- `audit_log`
- `app_settings`

Schema placeholders may include safe metadata fields such as IDs, names, timestamps, and configuration keys. They must not include plaintext password storage, private key storage, API tokens, or embedded credentials. Password-related fields, if present, must be clearly modeled for future password hashes only, not plaintext values.

## Deployment and Documentation Design

Deployment baseline will be documentation/placeholders only. It will not install, enable, start, stop, reload, or mutate system services.

Planned deployment documentation:

- `deploy/README.md` describes future systemd and HAProxy assets and states that examples are not installed automatically.
- `docs/ADR-0001-architecture.md` records the local-first single-node architecture decision.
- `docs/OPERATIONS.md` documents operational assumptions: same-host `ocserv`, HAProxy TLS termination, no root assumption, safe future service-control path, and no live mutation during baseline.
- `docs/SECURITY.md` documents authentication/RBAC expectations, audit expectations, secret-handling constraints, and privileged-operation boundaries.

## Agent Orchestration

Implementation should use focused ownership boundaries when useful:

- Backend agent for Go server, auth/RBAC placeholders, audit placeholders, SQLite connection placeholder, and ocserv interface placeholders.
- Frontend agent for SvelteKit skeleton and placeholder routes.
- SQLite persistence agent for the migration file.
- Deployment/docs agent for deployment notes and architecture/operations/security docs.
- Code/security review agent before final completion because the baseline touches auth/RBAC placeholders, persistence, ocserv integration boundaries, and deployment assumptions.

Agents must not edit the same files in parallel. Each handoff must include changed files, assumptions, tests or checks run, risks, and next steps.

## Testing and Verification

Planned verification:

- Run `go test ./...` from `backend/` after backend files exist.
- Run lightweight repository checks such as `git status --short`.
- Run frontend package checks only if dependencies are already available locally; do not install dependencies as part of this task.
- Do not require a live `ocserv` instance for any check.
- Do not run service-control commands or privileged commands.

## Known Gaps After Baseline

- No real authentication flow.
- No RBAC enforcement beyond placeholders.
- No real SQLite repository implementation or migration runner.
- No ocserv socket, `occtl`, or `ocpasswd` adapter implementation.
- No user/session/config/certificate management workflows.
- No systemd or HAProxy installation assets beyond documentation placeholders.
- No frontend/backend API integration beyond static placeholders and the backend health endpoint.

## Acceptance Criteria

- The requested top-level directories exist.
- Backend has a Go module, HTTP server, health endpoint, and safe placeholder packages/interfaces.
- Frontend has a hand-authored SvelteKit skeleton with layout, login, dashboard, and navigation placeholder pages.
- Migrations include initial schema placeholders for roles, admin users, audit log, and app settings.
- Docs include the architecture ADR, operations guide, and security guide.
- No privileged, destructive, or live `ocserv`/system service commands are executed.
- No secrets, plaintext passwords, private keys, tokens, or credentials are stored.
- Final output reports created structure, files added, checks run, known gaps, and recommended next development step.
