-- +goose Up
CREATE TABLE IF NOT EXISTS tenants (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO tenants (id, name) VALUES ('demo', 'Demo') ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS connects (
  tenant_id TEXT NOT NULL REFERENCES tenants(id),
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  version TEXT NOT NULL DEFAULT '',
  remote_addr TEXT NOT NULL DEFAULT '',
  first_connected_at TIMESTAMPTZ NOT NULL,
  last_seen_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS components (
  tenant_id TEXT NOT NULL REFERENCES tenants(id),
  connect_id TEXT NOT NULL,
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  transport TEXT NOT NULL,
  upstream_url TEXT NOT NULL DEFAULT '',
  public_url TEXT NOT NULL,
  registered_at TIMESTAMPTZ NOT NULL,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, connect_id, id),
  UNIQUE (tenant_id, name),
  FOREIGN KEY (tenant_id, connect_id) REFERENCES connects(tenant_id, id)
);
CREATE INDEX IF NOT EXISTS components_tenant_idx ON components(tenant_id);

CREATE TABLE IF NOT EXISTS device_codes (
  device_code_hash TEXT PRIMARY KEY,
  user_code_hash TEXT NOT NULL,
  tenant_id TEXT NOT NULL REFERENCES tenants(id),
  expires_at TIMESTAMPTZ NOT NULL,
  approved_at TIMESTAMPTZ,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS device_codes_user_code_idx ON device_codes(user_code_hash);

CREATE TABLE IF NOT EXISTS auth_tokens (
  token_hash TEXT PRIMARY KEY,
  token_type TEXT NOT NULL,
  tenant_id TEXT NOT NULL REFERENCES tenants(id),
  connect_id TEXT NOT NULL DEFAULT '',
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  replaced_by TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS auth_tokens_refresh_idx ON auth_tokens(token_hash) WHERE token_type = 'refresh';

CREATE TABLE IF NOT EXISTS audit_events (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  actor TEXT NOT NULL DEFAULT '',
  connect_id TEXT NOT NULL DEFAULT '',
  component_id TEXT NOT NULL DEFAULT '',
  transport TEXT NOT NULL DEFAULT '',
  method TEXT NOT NULL DEFAULT '',
  status INTEGER NOT NULL DEFAULT 0,
  latency_ms BIGINT NOT NULL DEFAULT 0,
  error_code TEXT NOT NULL DEFAULT '',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_events_tenant_created_idx ON audit_events(tenant_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS auth_tokens;
DROP TABLE IF EXISTS device_codes;
DROP TABLE IF EXISTS components;
DROP TABLE IF EXISTS connects;
DROP TABLE IF EXISTS tenants;
