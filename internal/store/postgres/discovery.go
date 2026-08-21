package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/r9s-ai/mcphub/internal/discovery"
)

func (s *ConnectStore) ListGroups(ctx context.Context, tenant string) ([]discovery.Group, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id,name,description,tags,is_default FROM tool_groups WHERE tenant_id=$1 ORDER BY name`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []discovery.Group{}
	for rows.Next() {
		var g discovery.Group
		var tags []byte
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &tags, &g.Default); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(tags, &g.Tags)
		out = append(out, g)
	}
	return out, rows.Err()
}
func (s *ConnectStore) CreateGroup(ctx context.Context, tenant string, g discovery.Group) error {
	tags, _ := json.Marshal(g.Tags)
	_, err := s.Pool.Exec(ctx, `INSERT INTO tenants(id,name) VALUES($1,$1) ON CONFLICT(id) DO NOTHING`, tenant)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO tool_groups(tenant_id,id,name,description,tags,is_default) VALUES($1,$2,$3,$4,$5,$6)`, tenant, g.ID, g.Name, g.Description, tags, g.Default)
	return err
}
func (s *ConnectStore) UpdateGroup(ctx context.Context, tenant string, g discovery.Group) error {
	tags, _ := json.Marshal(g.Tags)
	_, err := s.Pool.Exec(ctx, `UPDATE tool_groups SET name=$3,description=$4,tags=$5,is_default=$6,updated_at=now() WHERE tenant_id=$1 AND id=$2`, tenant, g.ID, g.Name, g.Description, tags, g.Default)
	return err
}
func (s *ConnectStore) DeleteGroup(ctx context.Context, tenant, id string) error {
	r, err := s.Pool.Exec(ctx, `DELETE FROM tool_groups WHERE tenant_id=$1 AND id=$2`, tenant, id)
	if err != nil {
		return err
	}
	if r.RowsAffected() == 0 {
		return fmt.Errorf("group_not_found")
	}
	return nil
}
func (s *ConnectStore) ListGroupTools(ctx context.Context, tenant, group string) ([]discovery.Tool, error) {
	rows, err := s.Pool.Query(ctx, `SELECT c.component_id,c.tool_name,c.description,c.input_schema,c.enabled FROM group_tools g JOIN tool_catalog c ON c.tenant_id=g.tenant_id AND c.component_id=g.component_id AND c.tool_name=g.tool_name WHERE g.tenant_id=$1 AND g.group_id=$2 ORDER BY c.tool_name`, tenant, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []discovery.Tool{}
	for rows.Next() {
		var t discovery.Tool
		var schema []byte
		if err := rows.Scan(&t.ComponentID, &t.Name, &t.Description, &schema, &t.Enabled); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(schema, &t.InputSchema)
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *ConnectStore) AttachTool(ctx context.Context, tenant, group string, t discovery.Tool) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO group_tools(tenant_id,group_id,component_id,tool_name) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, tenant, group, t.ComponentID, t.Name)
	return err
}
func (s *ConnectStore) DetachTool(ctx context.Context, tenant, group, component, tool string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM group_tools WHERE tenant_id=$1 AND group_id=$2 AND component_id=$3 AND tool_name=$4`, tenant, group, component, tool)
	return err
}
func (s *ConnectStore) ReplaceTools(ctx context.Context, tenant, component string, tools []discovery.Tool) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, t := range tools {
		schema, _ := json.Marshal(t.InputSchema)
		search := t.Name + " " + t.Description
		if _, err = tx.Exec(ctx, `INSERT INTO tool_catalog(tenant_id,component_id,tool_name,description,input_schema,search_text,enabled,last_seen_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,true,now(),now()) ON CONFLICT(tenant_id,component_id,tool_name) DO UPDATE SET description=EXCLUDED.description,input_schema=EXCLUDED.input_schema,search_text=EXCLUDED.search_text,enabled=true,last_seen_at=now(),updated_at=now()`, tenant, component, t.Name, t.Description, schema, search); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE tool_catalog SET enabled=false,updated_at=now() WHERE tenant_id=$1 AND component_id=$2 AND last_seen_at < now()`, tenant, component); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

var _ discovery.Store = (*ConnectStore)(nil)
