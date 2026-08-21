package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/r9s-ai/mcphub/internal/discovery"
)

type DiscoveryHandler struct{ service *discovery.Service }

func NewDiscovery(s *discovery.Service) *DiscoveryHandler { return &DiscoveryHandler{service: s} }
func (h *DiscoveryHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/groups", h.groups)
	mux.HandleFunc("/api/admin/groups/", h.group)
}
func tenantOf(r *http.Request) string {
	if v := r.URL.Query().Get("tenant_id"); v != "" {
		return v
	}
	return "demo"
}
func (h *DiscoveryHandler) groups(w http.ResponseWriter, r *http.Request) {
	tenant := tenantOf(r)
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"groups": h.service.ListGroups(tenant)})
	case http.MethodPost:
		var g discovery.Group
		if json.NewDecoder(r.Body).Decode(&g) != nil || g.ID == "" || g.Name == "" {
			http.Error(w, "invalid group", 400)
			return
		}
		if err := h.service.UpsertGroup(tenant, g); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		writeJSON(w, g)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
func (h *DiscoveryHandler) group(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/groups/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	tenant, id := tenantOf(r), parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			for _, g := range h.service.ListGroups(tenant) {
				if g.ID == id {
					writeJSON(w, g)
					return
				}
			}
			http.NotFound(w, r)
		case http.MethodPatch:
			var g discovery.Group
			if json.NewDecoder(r.Body).Decode(&g) != nil {
				http.Error(w, "invalid group", 400)
				return
			}
			g.ID = id
			if err := h.service.UpdateGroup(tenant, g); err != nil {
				http.Error(w, err.Error(), 404)
				return
			}
			writeJSON(w, g)
		case http.MethodDelete:
			if err := h.service.DeleteGroup(tenant, id); err != nil {
				http.Error(w, err.Error(), 404)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	if parts[1] != "tools" {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 2 && r.Method == http.MethodGet {
		tools, err := h.service.GroupTools(tenant, id)
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		writeJSON(w, map[string]any{"tools": tools})
		return
	}
	if len(parts) == 2 && r.Method == http.MethodPost {
		var in struct {
			ComponentID string `json:"component_id"`
			ToolName    string `json:"tool_name"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil || in.ComponentID == "" || in.ToolName == "" {
			http.Error(w, "invalid tool", 400)
			return
		}
		if err := h.service.AttachTool(tenant, id, discovery.Tool{ComponentID: in.ComponentID, Name: in.ToolName}); err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(parts) == 4 && r.Method == http.MethodDelete {
		if err := h.service.DetachTool(tenant, id, parts[2], parts[3]); err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.NotFound(w, r)
}
