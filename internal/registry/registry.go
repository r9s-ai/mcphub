package registry

import (
	"sort"
	"sync"
	"time"
)

const heartbeatTTL = 30 * time.Second

type ConnectInstance struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Version       string    `json:"version,omitempty"`
	Status        string    `json:"status"`
	ConnectedAt   time.Time `json:"connected_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	RemoteAddr    string    `json:"remote_addr,omitempty"`
}

type RegisteredComponent struct {
	ID            string    `json:"id"`
	ConnectID     string    `json:"connect_id"`
	Name          string    `json:"name"`
	Transport     string    `json:"transport"`
	UpstreamURL   string    `json:"upstream_url,omitempty"`
	Status        string    `json:"status"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	RegisteredAt  time.Time `json:"registered_at"`
	PublicURL     string    `json:"public_url"`
	LastError     string    `json:"last_error,omitempty"`
}

type Registry struct {
	mu         sync.RWMutex
	connects   map[string]*ConnectInstance
	components map[string]*RegisteredComponent
}

func New() *Registry {
	return &Registry{connects: map[string]*ConnectInstance{}, components: map[string]*RegisteredComponent{}}
}

func componentKey(connectID, componentID string) string { return connectID + ":" + componentID }

func (r *Registry) Register(connectID, connectName, version, componentID, componentName, transport, upstream, remote string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := r.connects[connectID]
	if c == nil {
		c = &ConnectInstance{ID: connectID, ConnectedAt: now}
		r.connects[connectID] = c
	}
	c.Name, c.Version, c.Status, c.LastHeartbeat, c.RemoteAddr = connectName, version, "online", now, remote
	key := componentKey(connectID, componentID)
	component := r.components[key]
	if component == nil {
		component = &RegisteredComponent{ID: componentID, ConnectID: connectID, RegisteredAt: now}
		r.components[key] = component
	}
	component.Name, component.Transport, component.UpstreamURL = componentName, transport, upstream
	component.Status, component.LastHeartbeat, component.PublicURL, component.LastError = "online", now, "/mcp/demo/"+componentID, ""
}

func (r *Registry) Heartbeat(connectID, componentID string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c := r.connects[connectID]; c != nil {
		c.Status, c.LastHeartbeat = "online", now
	}
	if component := r.components[componentKey(connectID, componentID)]; component != nil {
		component.Status, component.LastHeartbeat, component.LastError = "online", now, ""
	}
}

func (r *Registry) Disconnect(connectID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c := r.connects[connectID]; c != nil {
		c.Status = "offline"
	}
	for _, component := range r.components {
		if component.ConnectID == connectID {
			component.Status = "offline"
		}
	}
}

func (r *Registry) Expire(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.connects {
		if now.Sub(c.LastHeartbeat) > heartbeatTTL {
			c.Status = "offline"
		}
	}
	for _, component := range r.components {
		if now.Sub(component.LastHeartbeat) > heartbeatTTL {
			component.Status = "offline"
		}
	}
}

func (r *Registry) Snapshot() (connects []ConnectInstance, components []RegisteredComponent) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.connects {
		connects = append(connects, *c)
	}
	for _, c := range r.components {
		components = append(components, *c)
	}
	sort.Slice(connects, func(i, j int) bool { return connects[i].Name < connects[j].Name })
	sort.Slice(components, func(i, j int) bool { return components[i].Name < components[j].Name })
	return
}

func (r *Registry) Component(id string) (RegisteredComponent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.components {
		if c.ID == id {
			return *c, true
		}
	}
	return RegisteredComponent{}, false
}
