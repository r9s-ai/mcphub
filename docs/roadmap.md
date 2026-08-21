# MCPHub Roadmap

This document tracks the implementation roadmap for MCPHub: a local MCP connector paired with a cloud MCP Gateway.

Legend:

- `[x]` Implemented and verified in the current repository.
- `[~]` Partially implemented or available only as a development foundation.
- `[ ]` Not implemented.

## Current status

The repository currently provides a Phase 0/Phase 1 foundation. A local `mcp-connect` can register one stdio or Streamable HTTP component with `mcp-gateway`, forward basic request/response traffic over WebSocket, and expose in-memory status through the Admin API and dashboard.

The remaining MVP-critical work is true streaming and cancellation, reconnect handling, multi-component management, authentication, route isolation, and persistence.

## Phase 0 — Protocol and connector validation

### Foundation

- [x] Go repository structure with `mcp-connect`, `mcp-gateway`, connectors, tunnel, registry, Admin API, and shared protocol packages.
- [x] Versioned JSON WebSocket frame format (`protocol: "1"`).
- [x] Common connector interface with lifecycle, request handling, health, and metadata methods.
- [x] `StdioConnector` process startup, stdin/stdout forwarding, health check, and shutdown foundation.
- [x] `HTTPConnector` for already-running Streamable HTTP MCP servers.
- [x] Basic Gateway request routing through `/mcp/{tenant}/{component}`.
- [x] Basic WebSocket registration (`hello`) and heartbeat frames.
- [x] Basic stream ID based request correlation.
- [x] Unit tests for HTTP header/status forwarding and registry lifecycle.
- [x] English protocol and architecture documentation.

### Remaining Phase 0 work

- [~] Basic request/response forwarding works, and the tunnel now supports multiple response frames; protocol integration tests are still pending.
- [x] Forward response chunks as multiple frames and preserve `end_of_stream` semantics in the connector/daemon path.
- [~] Stream SSE and long-lived Streamable HTTP responses without buffering the complete body; Gateway end-to-end coverage is still pending.
- [x] Forward response chunks as soon as they are available from Streamable HTTP.
- [x] Add request cancellation frames and cancel upstream requests when the client disconnects.
- [ ] Add tunnel close/error control frames and wake all pending requests on disconnect.
- [ ] Add mock stdio and mock Streamable HTTP MCP servers for integration tests.
- [ ] Verify concurrent requests with out-of-order responses on one tunnel.

## Phase 1 — Single-user MVP

### Already available

- [x] `mcp-connect` command can start a stdio or Streamable HTTP connector.
- [x] `mcp-gateway` listens on port `3080` by default.
- [x] Admin frontend listens on port `3081` by default.
- [x] Makefile targets for install, development, testing, building, and running Gateway/Admin together.
- [x] pnpm-based Admin frontend with overview, connection list, component list, detail view, polling, and address copy.
- [~] In-memory connection/component registry with online/offline status and heartbeat expiry.
- [~] A single WebSocket can register one component; the data model reserves a future multi-component connection.

### CLI and local management

- [~] Add `mcp-connect login` with local configuration persistence, Device Code polling, and automatic daemon startup; production credential storage and user identity binding are still pending.
- [~] Add the initial `mcp-connect add`, `remove`, `list`, `status`, `test`, `enable`, and `disable` commands. The commands are available; richer output and service integration are still pending.
- [x] Add a persistent local configuration file.
- [x] Manage multiple components from one long-running `mcp-connect daemon` over one shared WebSocket tunnel.
- [ ] Support configured component headers and upstream bearer authentication through the CLI/config file.
- [~] Add a background/service mode for macOS and Linux. LaunchAgent and systemd user service file installation is available; platform-specific service activation and upgrade handling are still pending.

### Gateway, identity, and routing

- [~] Add Connect registration authentication. Static token validation, in-memory Device Code access tokens, refresh, and revoke endpoints are implemented; persistence, rotation, and per-tenant credentials are still pending.
- [ ] Add user/tenant identity and route access tokens.
- [~] Add tenant-aware registry keys and tenant/component Gateway routing; authenticated route authorization is still pending.
- [x] Enforce unique component names within one tenant during Gateway registration.
- [ ] Add token expiration, revocation, and per-component authorization.
- [~] Add `mcp-connect` automatic reconnect with backoff and re-registration in daemon mode; pending-request recovery and one shared tunnel are still pending.
- [ ] Preserve stable `connect_id` across reconnects.

### Persistence and distribution

- [~] Add PostgreSQL persistence and goose migrations. Connects, components, tenants, Device Code records, tokens, and audit schema are implemented; audit writes and full Admin repository reads are still pending.
- [~] Persist connects, components, tenants, and tokens. Gateway restores persisted Connect/component records on startup; Redis-backed live status overlay and complete history views remain pending.
- [ ] Add macOS/Linux installation scripts or packages.
- [ ] Publish a hosted Gateway demonstration environment.

### Phase 1 acceptance criteria

- [ ] One `mcp-connect` can manage and expose at least one stdio and one Streamable HTTP component concurrently.
- [ ] Both components are reachable through distinct authenticated public MCP URLs.
- [ ] Concurrent normal and streaming requests work over one tunnel.
- [ ] Gateway restart is recovered automatically by `mcp-connect`.
- [ ] An unavailable upstream returns a clear `upstream_unavailable` error and records the latest error.
- [ ] A route token cannot access another tenant or component.

## Phase 2 — Usability and security baseline

### Admin and operations

- [~] Read-only Admin dashboard and REST API exist under `/api/admin`.
- [ ] Add an `AdminAuthenticator` middleware boundary and enable Admin authentication.
- [ ] Add component health checks and expose the last health error.
- [ ] Add connection-change timestamps and richer status history.
- [ ] Add request timeout (3 seconds) and polling backoff (5 to 15 seconds) in the Admin frontend.
- [ ] Add frontend automated tests for loading, empty, error, polling, copy, and detail states.

### Streamable HTTP security

- [~] Loopback-only upstream validation is enabled by default, with explicit host allowlisting for development.
- [ ] Validate resolved IPs, not only the URL hostname.
- [ ] Block private networks, cloud metadata endpoints, unsafe ports, and unauthorized redirects.
- [ ] Add configurable HTTP header allowlists.
- [ ] Store upstream credentials securely and never expose them in Admin responses.
- [ ] Redact credentials and sensitive URL data in status and audit records.
- [ ] Separate Gateway-to-connect authentication from connect-to-upstream authentication.

### Limits, observability, and audit

- [ ] Add request body and response body limits.
- [ ] Add per-route rate limits and per-component concurrency limits.
- [~] Add structured audit logs with sensitive-field redaction. Gateway request audit persistence is wired to PostgreSQL; actor enrichment and full redaction policy are still pending.
- [ ] Add Gateway and `mcp-connect` metrics, health endpoints, and latency measurements.
- [ ] Add version checking and an upgrade mechanism for `mcp-connect`.

### Phase 2 acceptance criteria

- [ ] Admin status remains usable during temporary API failures and preserves the last successful snapshot.
- [ ] Unauthorized upstream URLs are rejected after DNS resolution and redirect checks.
- [ ] Credentials, authorization headers, and environment variables are absent from API responses and logs.
- [ ] Operators can identify component health, latency, errors, and connection history.

## Phase 3 — Secure Gateway and enterprise capabilities

- [ ] OAuth 2.1 compatible remote MCP access.
- [ ] User, team, service-account, and organization models.
- [ ] Scope and RBAC enforcement.
- [ ] Tool-level allow, deny, and approval policies.
- [ ] Human approval workflows.
- [ ] Upstream OAuth and mTLS.
- [ ] Credential rotation.
- [ ] Audit export, alerting, and policy version management.
- [ ] Enterprise SSO and SCIM.
- [ ] Self-hosted distribution.
- [ ] Multi-region high availability.

## Explicitly out of MVP

- [ ] Custom FRP/QUIC transport.
- [ ] Arbitrary TCP port forwarding.
- [ ] Gateway-side execution of local commands.
- [ ] Default proxying of arbitrary public URLs.
- [ ] Upstream OAuth, mTLS, and automatic credential rotation in the first release.
- [ ] Windows support in the first release.
- [ ] Billing and complex quota management.

## Recommended implementation order

1. Complete multi-frame tunnel streaming and cancellation.
2. Add reconnect handling and robust pending-request cleanup.
3. Move routing and Admin lookups to composite tenant/connect/component keys.
4. Implement local configuration and multi-component CLI management.
5. Add Connect authentication, route tokens, and PostgreSQL persistence.
6. Harden SSRF protection and upstream credential handling.
7. Add limits, audit logs, metrics, and production Admin authentication.
