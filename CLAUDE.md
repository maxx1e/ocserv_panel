# AGENTS.md

## Rules Files

The `.claude/rules/` directory contains reference files that agents MUST read before working in each domain:

- **`memory.md`** — Canonical index of all design specs and implementation plans in `docs/superpowers/`. Read this first before starting any feature work to find existing specs and plans that cover your area.
- **`project-structure.md`** — Go backend package map, key types, design principles, RBAC guard patterns, audit conventions, and testing checklist. Read before touching any backend code.
- **`svelteforge-admin-main.md`** — Frontend reference design (SvelteKit 5, Tailwind CSS v4, shadcn-svelte). Read before touching any frontend code.
- **`occtl-excamples.md`** — Real occtl JSON output samples. Read before parsing or testing any occtl command output.
- **`memory-tasks.md`** — Project progress snapshot (may be stale; verify against code).

**Agent workflow:**
1. Read `memory.md` to find specs/plans that cover your task.
2. Read the relevant spec file from `docs/superpowers/specs/` before reading code.
3. Read `project-structure.md` for backend work; `svelteforge-admin-main.md` for frontend work.
4. Implement using the plan from `docs/superpowers/plans/` if one exists.

## Project Intent

This repository implements a local Web UI admin panel for `ocserv`.

The product is a single-node, personal-use administration panel that runs on the same host as `ocserv`. It replaces routine manual operations with a local web dashboard while keeping the deployment lightweight and dependency-free.

## Architecture Decision

Build the system with:

- Go backend for API, system integration, RBAC, audit logging, and privileged operations
- SvelteKit frontend for dashboard and admin UI
- SQLite for local persistence
- tsdb for the monitoring metrics
- systemd service for runtime management
- HAProxy for TLS termination is optional deployment feature
- HTTP is a default connection protocol
- Select graph types based on whether the metric is time-based, distribution-based, or categorical.
- use md files from rules for additional reference.


Do not introduce external services unless explicitly requested.

## Allowed Dependencies
- `https://github.com/rs/zerolog` - for go lang logs
- `https://github.com/nakabonne/tstorage` - for the monitoring metrics
- `https://github.com/mattn/go-sqlite31` - for the sqllite integrations

## Scope

Core responsibilities:

- RBAC-based admin access
- User lifecycle management for password and certificate-based users
- Active session visibility and control through the `ocserv` management socket
- Historical session analytics
- Safe configuration editing with validation and backups
- Certificate visibility and lifecycle management
- Local dashboard without Prometheus, Grafana, or other monitoring stacks
- Dashboard should provide graphs with human read approach of the core metrics in the History tab: 
    - Performance metrics:
        - Bandwidth (inbound/outbound) for the network interface
        - Bandwidth (inbound/outbound) for the vpn interface
        - Bandwidth (inbound/outbound) for the user
    - Resource utilization
        - CPU
        - Memory
    - Usage metrics
        - Active sessions
        - Connection rate (new sessions/sec)
        - Concurrent sessions vs limit

Out of scope unless explicitly requested:

- Multi-node orchestration
- Distributed state
- Hosted SaaS behavior
- External monitoring stacks
- Container-first deployment
- Cloud-provider integrations

## Agent Rules

AI coding agents must:

- Preserve the single-node, local-first architecture
- Prefer small, auditable changes
- Treat privilege boundaries as security-critical
- Avoid broad refactors unless required for the requested change
- Keep backend, frontend, persistence, and system integration concerns separated
- Add tests for behavior that affects users, privileges, config parsing, session control, or persistence
- Document operational assumptions when touching systemd, sudo, HAProxy, ocserv sockets, certificates, or filesystem paths

AI coding agents must not:

- Add network dependencies or background services without approval
- Store secrets, private keys, passwords, or tokens in source control
- Execute destructive system commands without explicit user approval
- Assume root access is always available
- Bypass validation for ocserv configuration changes
- Modify live ocserv configuration without backup and validation paths
- Replace SQLite with another database without explicit approval

## Multi-Agent Development

When multiple AI coding agents work on this repository, split work by clear architecture and security boundaries. Every agent must preserve the local-first, single-node design and follow the same privilege, audit, and testing rules as a single agent.

### Recommended Agent Roles

Use focused agent roles when work benefits from separation:

- Backend API agent: Go APIs, RBAC, auth, audit logging, service interfaces, and API tests.
- Frontend admin UX agent: SvelteKit UI, admin workflows, frontend validation, destructive-action confirmations, and backend API integration.
- SQLite persistence agent: migrations, repositories, transactions, metadata schemas, and persistence tests.
- ocserv integration agent: ocserv management socket, `occtl`, `ocpasswd`, log parsing, command adapters, and fake-based integration tests.
- Config safety agent: configuration validation, backups, preview/diff flows, safe apply paths, and rollback assumptions.
- Security/RBAC agent: authorization checks, privilege boundaries, deny-path tests, audit coverage, secret handling, and sudo/root assumptions.
- Deployment/Ops agent: systemd, HAProxy, sudoers, filesystem permissions, packaging assets, and operational documentation.
- Certificate lifecycle agent: certificate metadata, expiry/revocation workflows, filesystem access, and private-key safety.
- Test/verification agent: unit, adapter, API, migration, and frontend validation tests without requiring live `ocserv`.
- Documentation/ADR agent: architecture notes, operational assumptions, privilege-boundary documentation, and ADR updates.
- Code review agent: final review for regressions, missing tests, unsafe assumptions, security risks, and architecture drift.

### Multi-Agent Workflow

Use multi-agent development only when work can be split into independent scopes. If the work is tightly coupled, use one agent and keep the change small.

Follow this workflow:

1. Define the feature or fix in one short objective.
2. Identify affected boundaries: backend, frontend, persistence, ocserv integration, deployment, security, docs, or tests.
3. Split work into agent scopes with clear ownership.
4. Assign each agent an objective, files or directories they may touch, files or directories they must not touch, required tests or verification, and expected handoff summary.
5. Run agents in parallel only when their scopes do not overlap.
6. Review each handoff before integrating changes.
7. Run full verification after integration.
8. Use a security/review agent for changes involving RBAC, credentials, certificates, config mutation, session termination, sudo, systemd, or filesystem permissions.
9. Update docs or ADRs when architecture, deployment, privilege boundaries, or operational assumptions change.

### When To Trigger Agents

Trigger specialized agents based on the primary risk of the task:

- Trigger the backend API agent when adding or changing Go API endpoints, auth, RBAC, audit logging, request validation, or service interfaces.
- Trigger the frontend admin UX agent when changing SvelteKit pages, forms, dashboard views, destructive-action flows, or API client behavior.
- Trigger the SQLite persistence agent when adding migrations, changing schemas, modifying repositories, or changing persisted audit/session/user/config/certificate metadata.
- Trigger the ocserv integration agent when touching `occtl`, `ocpasswd`, ocserv management socket behavior, log parsing, command execution, or external command adapters.
- Trigger the config safety agent when changing config parsing, validation, backup, restore, diff, preview, or apply behavior.
- Trigger the security/RBAC agent when changing permissions, authentication, authorization, session termination, password flows, certificate issuance/revocation, sudo boundaries, or secret handling.
- Trigger the deployment/Ops agent when changing systemd units, HAProxy config, sudoers rules, install scripts, packaging, runtime users, filesystem paths, or operational docs.
- Trigger the certificate lifecycle agent when changing certificate metadata, expiry checks, revocation flows, issuance flows, or certificate filesystem access.
- Trigger the test/verification agent before merging broad or risky changes, especially where live `ocserv` must be avoided in normal tests.
- Trigger the documentation/ADR agent when implementation changes architecture, scope, deployment assumptions, privilege boundaries, or operational behavior.
- Trigger the code review agent before merging security-sensitive, privileged, config-related, deployment-related, or cross-boundary changes.

### Agent Trigger Examples

Use one agent for focused work:

```text
Task: Add an API endpoint for listing active sessions.
Trigger:
- Backend API agent
- ocserv integration agent if socket parsing changes
- Security/RBAC agent for authorization review
- Test/verification agent before merge
```

Use parallel agents for independent work:

```text
Task: Add user management UI backed by existing APIs.
Parallel scopes:
- Frontend admin UX agent: SvelteKit user pages and form validation
- Test/verification agent: frontend validation tests
- Documentation/ADR agent: user workflow docs if behavior changes
```

Do not use parallel agents when they would edit the same migration, API contract, shared type, or privileged operation. In those cases, sequence the work and require explicit handoffs.

### Coordination Rules

- Assign each agent a narrow scope, expected files, and explicit non-goals before work starts.
- Avoid multiple agents editing the same file or migration sequence at the same time.
- Require security-sensitive changes to be reviewed by a separate security or review agent.
- Require handoffs to include changed files, assumptions, tests run, known risks, and operational impact.
- Keep backend, frontend, persistence, deployment, and ocserv integration concerns separated.
- Prefer small, reviewable changes over broad cross-cutting refactors.
- Use fakes or adapters for ocserv and system interactions in normal tests.
- Stop and reconcile design assumptions if agents discover conflicting approaches.

### Explicitly Forbidden

- Do not introduce external services, queues, hosted dependencies, observability stacks, or network control planes without explicit approval.
- Do not let frontend agents bypass backend APIs or access local files, sockets, commands, certificates, or secrets directly.
- Do not mutate live ocserv configuration without validation, backup, and a safe apply path.
- Do not run destructive system commands or service-control operations without explicit user approval.
- Do not store plaintext passwords, private keys, tokens, or credentials in source control or SQLite.
- Do not weaken RBAC, audit logging, input validation, or privilege checks for speed.
- Do not split work in a way that obscures ownership of migrations, audit events, privileged operations, or config changes.
- Do not resolve merge conflicts by discarding another agent's work without review.

## Backend Guidance

Use Go for:

- HTTP API
- Authentication and authorization
- RBAC enforcement
- SQLite access
- Audit logging
- Interactions with `occtl`, `ocpasswd`, ocserv config files, certificate files (cerbot), logs, and the ocserv management socket
- systemd-aware runtime behavior where needed

Design backend code so privileged operations are isolated behind explicit interfaces. Favor narrow functions that validate inputs, perform one operation, and return actionable errors.

All administrative actions that mutate users, sessions, configuration, certificates, or service state should produce audit records.

## Frontend Guidance

Use SvelteKit for:

- Admin dashboard
- User management UI
- Active sessions UI
- Historical analytics views
- Configuration editing workflows
- Certificate views and lifecycle controls
- Must use tailwind css framework
- MUST use reference design from the `svelteforge-admin-main` folder

The frontend should call backend APIs rather than shelling out or assuming direct filesystem access.

Design UI flows around safe administration:

- Show validation results before applying config changes
- Make destructive actions explicit
- Surface audit/history where relevant
- Prefer clear operational state over decorative complexity

## SQLite Guidance

SQLite is the local source of truth for application data such as:

- Admin users and RBAC metadata
- Managed user records
- Historical sessions
- Audit events
- Configuration backup metadata
- Certificate metadata

Use migrations for schema changes. Keep migration steps deterministic and safe to run during service startup or deployment.

Do not store raw private keys or plaintext passwords in SQLite.

## ocserv Integration Guidance

Interactions with `ocserv` must be defensive:

- Validate config before applying it
- Backup config before modifying it
- Prefer management socket/API operations for active sessions
- Treat command output from `occtl` and `ocpasswd` as unstable external input
- Handle missing sockets, missing binaries, permission errors, and service downtime gracefully

## Security Guidance

Security-sensitive areas include:

- RBAC checks
- Session termination
- User creation/deletion
- Password changes
- Certificate issuance/revocation
- Config edits
- systemd service control
- sudo/root boundary crossing

Every privileged backend action should have:

- Authorization check
- Input validation
- Clear error handling
- Audit logging
- Tests where practical

## Testing Guidance

Prefer tests around:

- RBAC decisions
- Config validation and backup logic
- SQLite migrations and repositories
- Audit logging
- ocserv command/socket adapters using fakes
- HTTP API behavior
- Frontend form validation for risky actions

Do not require a live `ocserv` instance for normal unit tests. Use fakes or adapters for system integration.

## Operational Assumptions

The app is expected to run on the same host as `ocserv`.

Runtime integration may require:

- A dedicated system user
- Limited sudo permissions
- Access to ocserv management socket
- Read/write access to selected ocserv config paths
- Read access to logs and certificate metadata
- HAProxy terminating TLS before requests reach the Go service

Any change to these assumptions should be documented.

## Preferred Project Shape

Expected high-level structure:

```text
backend/        Go API and system integration
frontend/       SvelteKit application
migrations/     SQLite migrations
deploy/         systemd, HAProxy, sudoers, packaging assets
docs/           Architecture, operations, and ADRs
```

Keep this structure unless implementation reveals a better minimal layout.

## Documentation Expectations

When changing architecture, privilege boundaries, deployment assumptions, or `ocserv` integration behavior, update documentation in the same change.

Prefer ADR-style notes for major decisions.
## Auto-Update Memory (MANDATORY)

**Update memory files AS YOU GO, not at the end.** When you learn something new, update immediately.

| Trigger                             | Action                                   |
| ----------------------------------- | ---------------------------------------- |
| User shares a fact about themselves | → Update `memory-profile.md`             |
| User states a preference            | → Update `memory-preferences.md`         |
| A decision is made                  | → Update `memory-decisions.md` with date |
| Completing substantive work         | → Add to `memory-sessions.md`            |

**Skip:** Quick factual questions, trivial tasks with no new info.

**DO NOT ASK. Just update the files when you learn something.**
