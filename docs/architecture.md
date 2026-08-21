# Architecture

MCPHub has two runtime components:

- `mcp-connect` runs on the user's machine and manages MCP connectors.
- `mcp-gateway` accepts public MCP requests and routes them through a persistent WebSocket tunnel.

The connector layer supports two transports:

- `StdioConnector` starts a local process and exchanges newline-delimited JSON-RPC over stdin/stdout.
- `HTTPConnector` proxies an already-running Streamable HTTP MCP server and preserves the relevant MCP headers and response status.

The Gateway registry keeps connection and component state in memory for the current process. It records registration time, last heartbeat, transport, upstream URL, public route, and the latest status.

The Admin API is deliberately separated from the Gateway data plane under `/api/admin`. Authentication is not enabled in development mode, but the route boundary is reserved for a future authenticator middleware.
