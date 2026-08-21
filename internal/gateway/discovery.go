package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/r9s-ai/mcphub/internal/discovery"
	"github.com/r9s-ai/mcphub/pkg/protocol"
)

type hubSession struct{ Identity discovery.Identity }
type hubRPC struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (g *Gateway) handleHub(w http.ResponseWriter, r *http.Request, tenant string) {
	var req hubRPC
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json-rpc", 400)
		return
	}
	sid := r.Header.Get("Mcp-Session-Id")
	if sid == "" {
		sid = time.Now().Format("20060102150405.000000000")
		w.Header().Set("Mcp-Session-Id", sid)
	}
	g.discovery.EnsureDefault(tenant)
	state, ok := g.registrySession(sid)
	if !ok {
		identity := discovery.Identity{TenantID: tenant, DefaultGroup: "default", ActiveGroup: "default"}
		if g.authStore != nil {
			authz := r.Header.Get("Authorization")
			if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			token := strings.TrimSpace(authz[len("Bearer "):])
			ti, err := g.authStore.ValidateAccessToken(r.Context(), hashTokenForGateway(token), time.Now())
			if err != nil || (ti.TenantID != "" && ti.TenantID != tenant) {
				http.Error(w, "invalid access token", http.StatusUnauthorized)
				return
			}
			identity.DefaultGroup = ti.DefaultGroupID
			identity.ActiveGroup = ti.DefaultGroupID
			identity.AllowedGroups = ti.AllowedGroupIDs
			if identity.DefaultGroup == "" {
				identity.DefaultGroup = "default"
				identity.ActiveGroup = "default"
			}
		}
		state = hubSession{Identity: identity}
		g.setRegistrySession(sid, state)
	}
	var result any
	var callErr error
	switch req.Method {
	case "initialize":
		result = map[string]any{"protocolVersion": "2025-03-26", "serverInfo": map[string]any{"name": "MCPHub Dynamic Discovery", "version": "0.1.0"}, "capabilities": map[string]any{"tools": map[string]any{}}}
	case "notifications/initialized":
		w.WriteHeader(202)
		return
	case "tools/list":
		result = map[string]any{"tools": metaTools()}
	case "tools/call":
		result, callErr = g.hubCall(r.Context(), sid, state, &req)
	default:
		callErr = fmt.Errorf("method_not_found")
	}
	if callErr != nil {
		writeHubError(w, req.ID, callErr)
		return
	}
	writeHubResult(w, req.ID, result)
}

func hashTokenForGateway(token string) string {
	// Auth stores use the URL-safe SHA-256 representation. Keep this local to
	// avoid exposing token material to the discovery package.
	h := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func (g *Gateway) registrySession(id string) (hubSession, bool) {
	v, ok := g.hubSessions.Load(id)
	if !ok {
		return hubSession{}, false
	}
	return v.(hubSession), true
}
func (g *Gateway) setRegistrySession(id string, s hubSession) { g.hubSessions.Store(id, s) }

func metaTools() []map[string]any {
	str := func() map[string]any { return map[string]any{"type": "string"} }
	obj := func() map[string]any { return map[string]any{"type": "object"} }
	return []map[string]any{
		{"name": "mcphub_search", "description": "Search tools in the active MCPHub group", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"query": str(), "limit": map[string]any{"type": "integer"}}}},
		{"name": "mcphub_get", "description": "Get a tool schema from the active group", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"component": str(), "tool": str()}, "required": []string{"component", "tool"}}},
		{"name": "mcphub_invoke", "description": "Invoke a tool from the active group", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"component": str(), "tool": str(), "arguments": obj()}, "required": []string{"component", "tool"}}},
		{"name": "mcphub_set_group", "description": "Switch to an authorized MCPHub group", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"group": str()}, "required": []string{"group"}}},
	}
}

func (g *Gateway) hubCall(ctx context.Context, sid string, state hubSession, req *hubRPC) (any, error) {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return nil, err
	}
	switch p.Name {
	case "mcphub_search":
		q, _ := p.Arguments["query"].(string)
		limit := 10
		if v, ok := p.Arguments["limit"].(float64); ok {
			limit = int(v)
		}
		return g.discovery.Search(ctx, discovery.SearchRequest{Identity: state.Identity, Query: q, Limit: limit})
	case "mcphub_get":
		c, _ := p.Arguments["component"].(string)
		t, _ := p.Arguments["tool"].(string)
		return g.discovery.Get(ctx, state.Identity, c, t)
	case "mcphub_set_group":
		group, _ := p.Arguments["group"].(string)
		next, gp, err := g.discovery.SetGroup(state.Identity, group)
		if err != nil {
			return nil, err
		}
		state.Identity = next
		g.setRegistrySession(sid, state)
		return gp, nil
	case "mcphub_invoke":
		c, _ := p.Arguments["component"].(string)
		t, _ := p.Arguments["tool"].(string)
		args, _ := p.Arguments["arguments"].(map[string]any)
		return g.discovery.Invoke(ctx, discovery.InvokeRequest{Identity: state.Identity, ComponentID: c, ToolName: t, Arguments: args})
	default:
		return nil, errors.New("tool_not_found")
	}
}

func writeHubResult(w http.ResponseWriter, id any, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}
func writeHubError(w http.ResponseWriter, id any, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(400)
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32000, "message": err.Error()}})
}

func (g *Gateway) invokeTool(ctx context.Context, tenant, component, tool string, args map[string]any) (any, error) {
	g.mu.RLock()
	s, ok := g.sessions[routeKey(tenant, component)]
	g.mu.RUnlock()
	if !ok {
		return nil, discovery.ErrToolNotFound
	}
	payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "mcphub", "method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
	id := time.Now().Format("20060102150405.000000000")
	frames, cleanup, err := s.request(ctx, protocol.Frame{Type: "request", StreamID: id, ComponentID: component, Method: "POST", Headers: map[string]string{"Content-Type": "application/json", "Accept": "application/json"}, Payload: payload})
	if err != nil {
		return nil, err
	}
	defer cleanup()
	res := <-frames
	if res.Error != "" {
		return nil, errors.New(res.Error)
	}
	var v any
	if err := json.Unmarshal(res.Payload, &v); err != nil {
		return string(res.Payload), nil
	}
	return v, nil
}
