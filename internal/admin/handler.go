package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/r9s-ai/mcphub/internal/registry"
)

type Handler struct{ registry *registry.Registry }

func New(r *registry.Registry) *Handler { return &Handler{registry: r} }
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/overview", h.overview)
	mux.HandleFunc("/api/admin/connects", h.connects)
	mux.HandleFunc("/api/admin/components", h.components)
	mux.HandleFunc("/api/admin/components/", h.component)
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
func (h *Handler) overview(w http.ResponseWriter, _ *http.Request) {
	connects, components := h.registry.Snapshot()
	onlineConnects, onlineComponents := 0, 0
	for _, c := range connects {
		if c.Status == "online" {
			onlineConnects++
		}
	}
	for _, c := range components {
		if c.Status == "online" {
			onlineComponents++
		}
	}
	writeJSON(w, map[string]any{"connect_total": len(connects), "connect_online": onlineConnects, "component_total": len(components), "component_online": onlineComponents, "component_offline": len(components) - onlineComponents, "last_updated": time.Now().UTC()})
}

type connectResponse struct {
	registry.ConnectInstance
	Components []registry.RegisteredComponent `json:"components"`
}

func (h *Handler) connects(w http.ResponseWriter, _ *http.Request) {
	connects, components := h.registry.Snapshot()
	result := make([]connectResponse, 0, len(connects))
	for _, c := range connects {
		item := connectResponse{ConnectInstance: c, Components: []registry.RegisteredComponent{}}
		for _, component := range components {
			if component.ConnectID == c.ID {
				item.Components = append(item.Components, component)
			}
		}
		result = append(result, item)
	}
	writeJSON(w, map[string]any{"connects": result})
}
func (h *Handler) components(w http.ResponseWriter, _ *http.Request) {
	_, components := h.registry.Snapshot()
	writeJSON(w, map[string]any{"components": components})
}
func (h *Handler) component(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/components/")
	component, ok := h.registry.Component(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, component)
}
