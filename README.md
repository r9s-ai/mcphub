# MCPHub

MCPHub is a local MCP connector and cloud Gateway. It supports local stdio MCP servers and already-running Streamable HTTP MCP servers over a shared WebSocket tunnel.

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

The current release implements the tunnel, routing, registration, heartbeat, and Admin observability foundation. Auth, RBAC, policy enforcement, rate limiting, audit persistence, and full observability are reserved for subsequent releases.

## Quick start

Start the Gateway:

```bash
make gateway
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

The public MCP endpoint is:

```text
POST http://127.0.0.1:3080/mcp/demo/remote-tools
```

## Admin center

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

Open `http://127.0.0.1:3081/admin`. The dashboard polls status every five seconds and currently uses an in-memory registry without Admin authentication.

The Admin frontend uses [pnpm](https://pnpm.io/). Install pnpm before running the frontend commands. The Makefile uses `pnpm install --ignore-scripts` because the Admin bundle does not require dependency build scripts.

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
docs/         English architecture and API documentation
deploy/       Deployment assets
```

## Current scope

This repository contains the Phase 0/Phase 1 foundation. Device authentication, PostgreSQL persistence, rate limiting, OAuth, and production Admin authorization are planned follow-up work. Streamable HTTP upstreams are restricted to loopback addresses unless an explicit `--allow-host` value is supplied.
