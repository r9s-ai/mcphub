# Group-scoped Dynamic Discovery

MCPHub exposes a virtual MCP server at `/mcp/{tenant}/hub`. The Hub keeps the
initial MCP tool list small and exposes four metadata tools instead of loading
every upstream tool into the client context:

- `mcphub_search`
- `mcphub_get`
- `mcphub_invoke`
- `mcphub_set_group`

Each tenant owns named groups. A group contains references to tools discovered
from registered components. A token has a default group and may be granted
additional groups. Search, schema retrieval, invocation, and group switching
are all checked against the token tenant and its allowed group set.

## Current implementation

The Gateway supports in-memory group and catalog management, tenant-aware Hub
invocation, and the following Admin endpoints:

```text
GET    /api/admin/groups
POST   /api/admin/groups
GET    /api/admin/groups/{group_id}
PATCH  /api/admin/groups/{group_id}
DELETE /api/admin/groups/{group_id}
GET    /api/admin/groups/{group_id}/tools
POST   /api/admin/groups/{group_id}/tools
DELETE /api/admin/groups/{group_id}/tools/{component_id}/{tool_name}
POST   /api/admin/catalog/components/{component_id}/refresh
```

When a component registers, the Gateway requests `tools/list` over the tunnel
and refreshes its catalog. Existing group-tool references are retained when a
tool temporarily disappears from an upstream catalog; unavailable tools are
simply omitted from discovery results until they return.

PostgreSQL migrations define durable `tool_catalog`, `tool_groups`,
`group_tools`, and `token_groups` tables. PostgreSQL access-token validation
now returns the token default group and authorized groups. The remaining work
is to replace the in-memory Discovery service with a repository backed by
these tables, add Redis catalog caching/locks, and complete the Admin UI.
