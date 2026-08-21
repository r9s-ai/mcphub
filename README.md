# MCPHub

MCPHub is a local MCP connector and cloud Gateway. It connects local stdio and Streamable HTTP MCP servers through a shared WebSocket tunnel, then provides tenant-aware routing, Group-scoped authorization, and context-efficient Dynamic Discovery to MCP clients.

**Connect local. Govern globally.**

## Architecture

```text
                         MCP Clients
                              │
            ┌─────────────────┼──────────────────┐
            │                 │                  │
         Claude             Codex              Agent
            │                 │                  │
            └─────────────────┼──────────────────┘
                              │
                              ▼
                     ┌──────────────────┐
                     │   MCP Gateway    │
                     │                  │
                     │ Auth             │
                     │ RBAC             │
                     │ Policy           │
                     │ Routing          │
                     │ Rate Limit       │
                     │ Audit            │
                     │ Observability    │
                     └────────┬─────────┘
                              │
                       MCP Tunnel
                              │
             ┌────────────────┼────────────────┐
             │                │                │
             ▼                ▼                ▼
        User Laptop        Server          Enterprise
             │                │                │
         MCP Agent        MCP Agent       MCP Agent
             │                │                │
        ┌────┼────┐       ┌───┼───┐       ┌───┼───┐
        ▼    ▼    ▼       ▼   ▼   ▼       ▼   ▼   ▼
       Git   DB  File    K8s  DB  Git    Jira DB  Git
```

The Gateway is the public control and data-plane entry point. It is responsible for request routing, authentication, policy enforcement, auditing, and operational visibility. A persistent MCP Tunnel connects the Gateway to one or more `mcp-connect` instances running in user laptops, servers, or enterprise environments.

Each `mcp-connect` instance manages local MCP components. Components may be stdio processes or already-running Streamable HTTP MCP servers. The same tunnel carries requests to multiple components and returns responses to the originating MCP client.

The current release implements the tunnel, routing, registration, heartbeat, persistent Group and Token state, tenant-aware route authentication, rate limiting, audit persistence foundation, and Dynamic Discovery. Full enterprise RBAC, OAuth, policy workflows, and complete observability remain subsequent work.

## Dynamic Discovery

The virtual Hub endpoint at `/mcp/{tenant}/hub` keeps client context small. Instead of exposing every upstream Tool Schema during initialization, it exposes four stable meta-tools:

- `mcphub_search`: find relevant tools in the active authorized Group.
- `mcphub_get`: load one Tool Schema only when it is needed.
- `mcphub_invoke`: invoke the selected upstream tool through the Gateway.
- `mcphub_set_group`: switch between Groups allowed for the current Token.

This search → schema → invoke flow lets one Hub aggregate a large catalog without placing the complete catalog in every Agent request.

## Quick start

Start the Gateway:

```bash
make gateway
```

For development, Connect authentication is disabled by default. To require a shared registration token:

```bash
MCP_GATEWAY_CONNECT_TOKEN=change-me make gateway
mcp-connect login --token change-me
```

Proxy a Streamable HTTP MCP server:

```bash
go run ./cmd/mcp-connect \
  --gateway ws://127.0.0.1:3080/tunnel \
  --connect-id laptop-001 \
  --connect-name my-laptop \
  --name remote-tools \
  --transport streamable-http \
  --url https://127.0.0.1:9000/mcp \
```

Proxy a stdio MCP server:

```bash
go run ./cmd/mcp-connect \
  --gateway ws://127.0.0.1:3080/tunnel \
  --name local-tools \
  --transport stdio \
  --command "node ./mock-mcp.js"
```

For local multi-component management, run the single CLI in daemon mode:

```bash
mcp-connect daemon --gateway ws://127.0.0.1:3080/tunnel --connect-name my-laptop
mcp-connect add github --transport stdio --command "npx @modelcontextprotocol/server-github"
mcp-connect add remote-tools --transport streamable-http --url http://127.0.0.1:8080/mcp
mcp-connect list
mcp-connect status
mcp-connect test remote-tools
mcp-connect disable github
mcp-connect enable github
mcp-connect remove remote-tools
```

The daemon stores its configuration under the platform user config directory and exposes a local Unix Socket control API. The existing flag-based one-component invocation remains supported for compatibility.

`login` can bootstrap the same daemon automatically:

```bash
mcp-connect login --gateway ws://127.0.0.1:3080/tunnel --connect-name my-laptop
```

Without `--token`, the CLI starts the Device Code flow, prints a verification URL and user code, and polls until browser confirmation is complete. The current development Gateway keeps device approvals in memory and uses a minimal confirmation page; production authentication and secure credential storage remain follow-up work.

Install a user-level daemon service on macOS or Linux:

```bash
mcp-connect install-service
# Linux: optionally enable and start immediately
mcp-connect install-service --start
```

Remove the generated service file with `mcp-connect uninstall-service`.

The public MCP endpoint is:

```text
POST http://127.0.0.1:3080/mcp/demo/remote-tools
```

Component names are unique within a tenant. Registering the same name from another connection is rejected instead of replacing the existing route.

## Web and Admin center

The Gateway serves the public MCPHub landing page at `/` and the management application under `/admin` from `web/admin/dist` by default. Override the production build directory with `MCP_WEB_DIR`.

When `MCP_ADMIN_TOKEN` is configured, open `/admin/login` and enter the Token. The browser stores it only in the current tab and sends it as a Bearer credential to Admin APIs. Without `MCP_ADMIN_TOKEN`, the UI detects development mode and enters the console directly.

The read-only Admin API is available at:

```text
GET http://127.0.0.1:3080/api/admin/overview
GET http://127.0.0.1:3080/api/admin/connects
GET http://127.0.0.1:3080/api/admin/components
```

Run the React admin application:

```bash
make install
make admin
```

During Vite development, open `http://127.0.0.1:3081/`; API and MCP requests are proxied to the Gateway. For a production-style local run, build the Web application and open the Gateway directly:

```bash
make build-admin
make gateway
# http://127.0.0.1:3080/
```

The Admin frontend uses [pnpm](https://pnpm.io/). Install pnpm before running the frontend commands. The Makefile uses `pnpm install --ignore-scripts` because the Admin bundle does not require dependency build scripts.

Start the local PostgreSQL and Redis dependencies with:

```bash
make infra-up
make migrate
```

Set `MCP_STORAGE=postgres`, `DATABASE_URL`, and `REDIS_URL` when running the Gateway against persistent storage. Without these variables the Gateway keeps using its in-memory development registry.

To start the Gateway and Admin together:

```bash
make dev
```

Run `make help` to see all available development commands.

## Repository layout

```text
cmd/          Executable entrypoints
internal/     Gateway, connector, registry, and Admin implementation
pkg/          Shared wire protocol
web/admin/    React + TypeScript Admin application
docs/         English architecture, API, protocol, and roadmap documentation
deploy/       Deployment assets
```

## Current scope

This repository contains the Phase 0/Phase 1 foundation. Device authentication, PostgreSQL persistence, rate limiting, OAuth, and production Admin authorization are planned follow-up work. Streamable HTTP upstreams are restricted to loopback addresses unless an explicit `--allow-host` value is supplied.

See [`docs/roadmap.md`](docs/roadmap.md) for the detailed implementation status and next steps.
