package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/r9s-ai/mcphub/internal/registry"
	"github.com/r9s-ai/mcphub/internal/store"
)

type ConnectStore struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, dsn string) (*ConnectStore, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &ConnectStore{Pool: pool}, nil
}
func (s *ConnectStore) Close() {
	if s != nil && s.Pool != nil {
		s.Pool.Close()
	}
}
func Migrate(ctx context.Context, dsn, dir string) error {
	db, err := sqlOpen(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	goose.SetDialect("postgres")
	return goose.Up(db, dir)
}
func sqlOpen(ctx context.Context, dsn string) (*sql.DB, error) { _ = ctx; return sql.Open("pgx", dsn) }
func (s *ConnectStore) RegisterConnect(ctx context.Context, in store.RegisterConnectInput, at time.Time) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO tenants(id,name) VALUES($1,$1) ON CONFLICT(id) DO NOTHING`, in.TenantID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO connects(tenant_id,id,name,version,remote_addr,first_connected_at,last_seen_at) VALUES($1,$2,$3,$4,$5,$6,$6) ON CONFLICT(tenant_id,id) DO UPDATE SET name=EXCLUDED.name,version=EXCLUDED.version,remote_addr=EXCLUDED.remote_addr,last_seen_at=EXCLUDED.last_seen_at`, in.TenantID, in.ConnectID, in.ConnectName, in.Version, in.RemoteAddr, at)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO components(tenant_id,connect_id,id,name,transport,upstream_url,public_url,registered_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(tenant_id,connect_id,id) DO UPDATE SET name=EXCLUDED.name,transport=EXCLUDED.transport,upstream_url=EXCLUDED.upstream_url,public_url=EXCLUDED.public_url`, in.TenantID, in.ConnectID, in.ComponentID, in.ComponentName, in.Transport, in.UpstreamURL, "/mcp/"+in.TenantID+"/"+in.ComponentID, at)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *ConnectStore) Heartbeat(ctx context.Context, tenant, connect, component string, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `UPDATE connects SET last_seen_at=$4 WHERE tenant_id=$1 AND id=$2`, tenant, connect, component, at)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `UPDATE components SET last_error='' WHERE tenant_id=$1 AND connect_id=$2 AND id=$3`, tenant, connect, component)
	return err
}
func (s *ConnectStore) Disconnect(ctx context.Context, tenant, connect string, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `UPDATE connects SET last_seen_at=$3 WHERE tenant_id=$1 AND id=$2`, tenant, connect, at)
	return err
}
func (s *ConnectStore) ListConnects(ctx context.Context, tenant string) ([]registry.ConnectInstance, error) {
	q := `SELECT tenant_id,id,name,version,remote_addr,first_connected_at,last_seen_at,CASE WHEN last_seen_at > now()-interval '30 seconds' THEN 'online' ELSE 'offline' END FROM connects`
	args := []any{}
	if tenant != "" {
		q += " WHERE tenant_id=$1"
		args = append(args, tenant)
	}
	q += " ORDER BY name"
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []registry.ConnectInstance{}
	for rows.Next() {
		var c registry.ConnectInstance
		if err := rows.Scan(&c.TenantID, &c.ID, &c.Name, &c.Version, &c.RemoteAddr, &c.ConnectedAt, &c.LastHeartbeat, &c.Status); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *ConnectStore) ListComponents(ctx context.Context, tenant string) ([]registry.RegisteredComponent, error) {
	q := `SELECT tenant_id,connect_id,id,name,transport,upstream_url,status,registered_at,last_error FROM (SELECT c.*,CASE WHEN x.last_seen_at > now()-interval '30 seconds' THEN 'online' ELSE 'offline' END status FROM components c JOIN connects x USING(tenant_id,connect_id)) q`
	args := []any{}
	if tenant != "" {
		q += " WHERE tenant_id=$1"
		args = append(args, tenant)
	}
	q += " ORDER BY name"
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []registry.RegisteredComponent{}
	for rows.Next() {
		var c registry.RegisteredComponent
		if err := rows.Scan(&c.TenantID, &c.ConnectID, &c.ID, &c.Name, &c.Transport, &c.UpstreamURL, &c.Status, &c.RegisteredAt, &c.LastError); err != nil {
			return nil, err
		}
		c.PublicURL = "/mcp/" + c.TenantID + "/" + c.ID
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *ConnectStore) GetComponent(ctx context.Context, tenant, id string) (registry.RegisteredComponent, error) {
	cs, err := s.ListComponents(ctx, tenant)
	if err != nil {
		return registry.RegisteredComponent{}, err
	}
	for _, c := range cs {
		if c.ID == id {
			return c, nil
		}
	}
	return registry.RegisteredComponent{}, fmt.Errorf("component not found")
}

var _ store.ConnectStore = (*ConnectStore)(nil)
