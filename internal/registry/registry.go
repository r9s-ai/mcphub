package registry

import (
	"errors"
	"sort"
	"sync"
	"time"
)

var ErrComponentNameTaken = errors.New("component name is already registered in tenant")

const heartbeatTTL = 30 * time.Second

type ConnectInstance struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	Name          string    `json:"name"`
	Version       string    `json:"version,omitempty"`
	Status        string    `json:"status"`
	ConnectedAt   time.Time `json:"connected_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	RemoteAddr    string    `json:"remote_addr,omitempty"`
}

type RegisteredComponent struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
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

func connectKey(tenantID, connectID string) string { return tenantID + ":" + connectID }
func componentKey(tenantID, connectID, componentID string) string {
	return tenantID + ":" + connectID + ":" + componentID
}

func (r *Registry) Register(tenantID, connectID, connectName, version, componentID, componentName, transport, upstream, remote string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.components {
		if existing.TenantID == tenantID && existing.Name == componentName && (existing.ConnectID != connectID || existing.ID != componentID) {
			return ErrComponentNameTaken
		}
	}
	c := r.connects[connectKey(tenantID, connectID)]
	if c == nil {
		c = &ConnectInstance{ID: connectID, TenantID: tenantID, ConnectedAt: now}
		r.connects[connectKey(tenantID, connectID)] = c
	}
	c.Name, c.Version, c.Status, c.LastHeartbeat, c.RemoteAddr = connectName, version, "online", now, remote
	key := componentKey(tenantID, connectID, componentID)
	component := r.components[key]
	if component == nil {
		component = &RegisteredComponent{ID: componentID, TenantID: tenantID, ConnectID: connectID, RegisteredAt: now}
		r.components[key] = component
	}
	component.Name, component.Transport, component.UpstreamURL = componentName, transport, upstream
	component.TenantID = tenantID
	component.Status, component.LastHeartbeat, component.PublicURL, component.LastError = "online", now, "/mcp/"+tenantID+"/"+componentID, ""
	return nil
}

func (r *Registry) Heartbeat(tenantID, connectID, componentID string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c := r.connects[connectKey(tenantID, connectID)]; c != nil {
		c.Status, c.LastHeartbeat = "online", now
	}
	if component := r.components[componentKey(tenantID, connectID, componentID)]; component != nil {
		component.Status, component.LastHeartbeat, component.LastError = "online", now, ""
	}
}

func (r *Registry) Disconnect(tenantID, connectID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c := r.connects[connectKey(tenantID, connectID)]; c != nil {
		c.Status = "offline"
	}
	for _, component := range r.components {
		if component.TenantID == tenantID && component.ConnectID == connectID {
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
