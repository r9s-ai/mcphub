# Tunnel Protocol

The first protocol version uses JSON frames over a persistent WebSocket.

Control frames include:

- `hello`: registers a connection and component.
- `heartbeat`: refreshes the connection and component last-seen time.
- `request`: forwards an MCP request.
- `response`: returns an MCP response, including status, headers, payload, and stream completion.

The protocol carries a `stream_id` so multiple MCP requests can share one tunnel and complete out of order.
