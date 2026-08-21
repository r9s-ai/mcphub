# Architecture

MCPHub has two runtime components:

- `mcp-connect` runs on the user's machine. Its single CLI can run a local daemon that manages multiple MCP connectors.
- `mcp-gateway` accepts public MCP requests and routes them through a persistent WebSocket tunnel.

The connector layer supports two transports:

- `StdioConnector` starts a local process and exchanges newline-delimited JSON-RPC over stdin/stdout.
- `HTTPConnector` proxies an already-running Streamable HTTP MCP server and preserves the relevant MCP headers and response status.

The daemon stores local component configuration and exposes a Unix Socket control API to the CLI. It uses one persistent WebSocket tunnel for the configured components, with automatic reconnect and heartbeat registration.

The Gateway registry keeps connection and component state in memory for the current process. It records registration time, last heartbeat, transport, upstream URL, public route, and the latest status.

The Admin API is deliberately separated from the Gateway data plane under `/api/admin`. Authentication is not enabled in development mode, but the route boundary is reserved for a future authenticator middleware.

Component names are unique within a tenant. A duplicate registration is rejected by the Gateway so an existing public route cannot be silently replaced.
