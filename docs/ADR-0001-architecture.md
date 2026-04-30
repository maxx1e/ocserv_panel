# ADR-0001: Local-First Single-Node Architecture

## Status

Accepted.

## Context

The project provides a personal-use Web UI admin panel for an `ocserv` server running on the same host. It replaces routine manual administration with a local dashboard while avoiding external services and distributed state.

## Decision

Use the following architecture:

- Custom Go backend for HTTP APIs, authentication, RBAC, audit logging, SQLite access, and isolated privileged-operation interfaces.
- SvelteKit frontend for the admin dashboard and operator workflows.
- SQLite for local persistence, using the approved pure-Go `modernc.org/sqlite` driver so builds do not require CGO or a system C compiler.
- systemd for runtime management.
- HAProxy for TLS termination in front of the Go backend.

The backend owns all access to local files, sockets, commands, certificates, and service-control boundaries. The frontend communicates only through backend APIs.

The application remains a same-host, same-origin deployment: browser requests use `/api`, HAProxy terminates production TLS before forwarding to the loopback Go service, and local development may use the Vite proxy to `127.0.0.1:8080`. CORS is not required for the intended deployment model.

PocketBase was considered for rapid CRUD/auth scaffolding and rejected. The project requires narrow privileged-operation boundaries, explicit RBAC and audit behavior, local `ocserv` integration adapters, and deployment assumptions that are clearer in a custom Go backend. No PocketBase runtime, generated admin UI, or external service dependency is part of the accepted architecture.

## Consequences

- The system stays lightweight and suitable for a single host.
- Normal tests must use fakes or adapters and must not require a live `ocserv` instance.
- Privileged operations require explicit interfaces, authorization checks, validation, error handling, and audit logging.
- Multi-node orchestration, hosted dependencies, external monitoring stacks, queues, and cloud control planes remain out of scope unless explicitly requested.
- The authenticated dashboard slice may expose local dashboard and historical analytics through authenticated APIs, but it must not call live `ocserv` or mutate system/config/certificate state until those integration paths are explicitly implemented with documented safety controls.
