# Admin API

All endpoints are read-only in the current development release.

## Overview

`GET /api/admin/overview`

Returns aggregate connection and component counts.

## Connections

`GET /api/admin/connects`

Returns connection instances grouped with their registered MCP components.

## Components

`GET /api/admin/components`

Returns all registered MCP components.

`GET /api/admin/components/{component_id}`

Returns one component by ID.

The API never returns upstream authorization headers, tokens, or environment variables.
