-- +goose Up
CREATE TABLE IF NOT EXISTS tool_catalog (
  tenant_id TEXT NOT NULL,
  component_id TEXT NOT NULL,
  tool_name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  input_schema JSONB NOT NULL DEFAULT '{}'::jsonb,
  search_text TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_error TEXT NOT NULL DEFAULT '',
  catalog_version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, component_id, tool_name)
);
CREATE INDEX IF NOT EXISTS tool_catalog_search_idx ON tool_catalog(tenant_id, search_text);

CREATE TABLE IF NOT EXISTS tool_groups (
  tenant_id TEXT NOT NULL REFERENCES tenants(id),
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS group_tools (
  tenant_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  component_id TEXT NOT NULL,
  tool_name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, group_id, component_id, tool_name),
  FOREIGN KEY (tenant_id, group_id) REFERENCES tool_groups(tenant_id, id),
  FOREIGN KEY (tenant_id, component_id, tool_name) REFERENCES tool_catalog(tenant_id, component_id, tool_name)
);

CREATE TABLE IF NOT EXISTS token_groups (
  token_hash TEXT NOT NULL,
  tenant_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (token_hash, tenant_id, group_id),
  FOREIGN KEY (tenant_id, group_id) REFERENCES tool_groups(tenant_id, id)
);

ALTER TABLE auth_tokens ADD COLUMN IF NOT EXISTS default_group_id TEXT NOT NULL DEFAULT 'default';

-- +goose Down
ALTER TABLE auth_tokens DROP COLUMN IF EXISTS default_group_id;
DROP TABLE IF EXISTS token_groups;
DROP TABLE IF EXISTS group_tools;
DROP TABLE IF EXISTS tool_groups;
DROP TABLE IF EXISTS tool_catalog;
