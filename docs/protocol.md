# Tunnel Protocol

The first protocol version uses JSON frames over a persistent WebSocket.

Control frames include:

- `hello`: registers a connection and component.
- `heartbeat`: refreshes the connection and component last-seen time.
- `request`: forwards an MCP request.
- `response`: returns an MCP response, including status, headers, payload, and stream completion.
- `cancel`: cancels an in-flight request by `stream_id`.

The protocol carries a `stream_id` so multiple MCP requests can share one tunnel and complete out of order. `hello` includes a `gateway_token` when Gateway Connect authentication is enabled. The Gateway accepts either the configured static token or an unexpired Device Code access token. `hello` and `heartbeat` include `tenant_id`; development mode defaults this value to `demo`, and Gateway routes are keyed by `tenant_id/component_id`.

The development auth endpoints also expose OAuth-style token refresh and revoke operations at `/api/auth/token` and `/api/auth/revoke`. Tokens are currently held in Gateway memory.
