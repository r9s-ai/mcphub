package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/r9s-ai/mcphub/internal/admin"
	"github.com/r9s-ai/mcphub/internal/auth"
	"github.com/r9s-ai/mcphub/internal/registry"
	"github.com/r9s-ai/mcphub/internal/store"
	"github.com/r9s-ai/mcphub/pkg/protocol"
)

type session struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
	mu      sync.Mutex
	pending map[string]chan protocol.Frame
	closed  chan struct{}
}

func newSession(c *websocket.Conn) *session {
	return &session{conn: c, pending: map[string]chan protocol.Frame{}, closed: make(chan struct{})}
}
func (s *session) write(f protocol.Frame) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, b)
}
func (s *session) request(ctx context.Context, f protocol.Frame) (<-chan protocol.Frame, func(), error) {
	ch := make(chan protocol.Frame, 8)
	s.mu.Lock()
	s.pending[f.StreamID] = ch
	s.mu.Unlock()
	if err := s.write(f); err != nil {
		s.mu.Lock()
		delete(s.pending, f.StreamID)
		s.mu.Unlock()
		return nil, nil, err
	}
	cleanup := func() { s.mu.Lock(); delete(s.pending, f.StreamID); s.mu.Unlock() }
	go func() {
		select {
		case <-ctx.Done():
			_ = s.write(protocol.Frame{Type: "cancel", StreamID: f.StreamID})
		case <-s.closed:
		}
	}()
	return ch, cleanup, nil
}
func (s *session) readLoop(onHello func(protocol.Frame, *session)) {
	defer func() {
		s.mu.Lock()
		for _, ch := range s.pending {
			close(ch)
		}
		s.pending = map[string]chan protocol.Frame{}
		s.mu.Unlock()
		close(s.closed)
	}()
	for {
		_, b, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		var f protocol.Frame
		if json.Unmarshal(b, &f) != nil {
			return
		}
		if f.Type == "hello" || f.Type == "heartbeat" {
			onHello(f, s)
			continue
		}
		s.mu.Lock()
		ch := s.pending[f.StreamID]
		s.mu.Unlock()
		if ch != nil {
			select {
			case ch <- f:
			default:
			}
		}
	}
}

type Gateway struct {
	mu            sync.RWMutex
	sessions      map[string]*session
	registry      *registry.Registry
	upgrader      websocket.Upgrader
	connectToken  string
	deviceAuth    *auth.DeviceAuth
	connectStore  store.ConnectStore
	presenceStore store.PresenceStore
	auditStore    store.AuditStore
}

func routeKey(tenant, component string) string { return tenant + ":" + component }

func New() *Gateway {
	return NewWithToken(os.Getenv("MCP_GATEWAY_CONNECT_TOKEN"))
}
func NewWithToken(connectToken string) *Gateway {
	return NewWithOptions(connectToken, "http://127.0.0.1:3080")
}
func NewWithOptions(connectToken, publicURL string) *Gateway {
	return NewWithStores(connectToken, publicURL, nil, nil)
}
func NewWithStores(connectToken, publicURL string, cs store.ConnectStore, ps store.PresenceStore, authStores ...store.AuthStore) *Gateway {
	var as store.AuthStore
	if len(authStores) > 0 {
		as = authStores[0]
	}
	deviceAuth := auth.NewDeviceAuthWithStores(publicURL, as, ps)
	var audit store.AuditStore
	if v, ok := cs.(store.AuditStore); ok {
		audit = v
	}
	g := &Gateway{sessions: map[string]*session{}, registry: registry.New(), connectToken: connectToken, deviceAuth: deviceAuth, connectStore: cs, presenceStore: ps, auditStore: audit, upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}}
	if cs != nil {
		go g.restore(context.Background())
	}
	if ps != nil {
		go g.presenceLoop()
	}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for now := range ticker.C {
			g.registry.Expire(now)
		}
	}()
	return g
}
func (g *Gateway) presenceLoop() {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for now := range t.C {
		_ = now
		connects, components := g.registry.Snapshot()
		for _, c := range connects {
			online, err := g.presenceStore.ConnectOnline(context.Background(), c.TenantID, c.ID)
			if err == nil && !online {
				g.registry.Disconnect(c.TenantID, c.ID)
			}
		}
		for _, c := range components {
			online, err := g.presenceStore.ComponentOnline(context.Background(), c.TenantID, c.ConnectID, c.ID)
			if err == nil && !online {
				g.registry.DisconnectComponent(c.TenantID, c.ConnectID, c.ID)
			}
		}
	}
}
func (g *Gateway) restore(ctx context.Context) {
	cs, err := g.connectStore.ListConnects(ctx, "")
	if err == nil {
		for _, c := range cs {
			g.registry.RestoreConnect(c)
		}
	}
	comps, err := g.connectStore.ListComponents(ctx, "")
	if err == nil {
		for _, c := range comps {
			g.registry.RestoreComponent(c)
		}
	}
}
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp/", g.handleMCP)
	mux.HandleFunc("/tunnel", g.handleTunnel)
	admin.New(g.registry).Register(mux)
	g.deviceAuth.Register(mux)
	return mux
}
func (g *Gateway) handleMCP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "mcp" {
		http.NotFound(w, r)
		return
	}
	tenant, component := parts[1], parts[2]
	g.mu.RLock()
	s, ok := g.sessions[routeKey(tenant, component)]
	g.mu.RUnlock()
	if !ok {
		http.Error(w, "component not connected", 503)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	h := map[string]string{}
	for _, k := range []string{"Authorization", "Accept", "Content-Type", "Mcp-Session-Id"} {
		if v := r.Header.Get(k); v != "" {
			h[k] = v
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	started := time.Now()
	auditStatus := 0
	auditError := ""
	defer func() {
		if g.auditStore != nil {
			_ = g.auditStore.RecordAudit(context.Background(), store.AuditEvent{TenantID: tenant, ComponentID: component, Method: r.Method, Status: auditStatus, Latency: time.Since(started), ErrorCode: auditError}, time.Now())
		}
	}()
	id := time.Now().Format("20060102150405.000000000")
	frames, cleanup, err := s.request(ctx, protocol.Frame{Type: "request", StreamID: id, ComponentID: component, Method: r.Method, Headers: h, Payload: body})
	if err != nil {
		auditError = "tunnel_request_failed"
		http.Error(w, err.Error(), 502)
		return
	}
	defer cleanup()
	flusher, _ := w.(http.Flusher)
	wroteHeader := false
	for {
		select {
		case res, ok := <-frames:
			if !ok {
				auditError = "tunnel_closed"
				if !wroteHeader {
					http.Error(w, "tunnel closed", 502)
				}
				return
			}
			if res.Error != "" && res.Status == 0 {
				auditError = res.Error
				if !wroteHeader {
					http.Error(w, res.Error, 502)
				}
				return
			}
			if !wroteHeader {
				for k, v := range res.Headers {
					w.Header().Set(k, v)
				}
				status := res.Status
				if status == 0 {
					status = 200
				}
				w.WriteHeader(status)
				auditStatus = status
				wroteHeader = true
			}
			if len(res.Payload) > 0 {
				_, _ = w.Write(res.Payload)
				if flusher != nil {
					flusher.Flush()
				}
			}
			if res.EndOfStream {
				return
			}
		case <-ctx.Done():
			if !wroteHeader {
				http.Error(w, ctx.Err().Error(), 502)
			}
			return
		}
	}
}

func (g *Gateway) handleTunnel(w http.ResponseWriter, r *http.Request) {
	conn, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s := newSession(conn)
	defer conn.Close()
	connectID, tenantID := "", "demo"
	s.readLoop(func(f protocol.Frame, current *session) {
		if f.Type == "hello" && ((g.connectToken != "" && f.GatewayToken != g.connectToken) && !g.deviceAuth.Validate(f.GatewayToken)) {
			_ = current.write(protocol.Frame{Type: "error", Error: "invalid connect token"})
			_ = current.conn.Close()
			return
		}
		if f.Type == "heartbeat" {
			if f.TenantID != "" {
				tenantID = f.TenantID
			}
			id := f.ConnectID
			if id == "" {
				id = connectID
			}
			if tenantID == "" {
				tenantID = "demo"
			}
			g.registry.Heartbeat(tenantID, id, f.ComponentID, time.Now().UTC())
			if g.connectStore != nil {
				_ = g.connectStore.Heartbeat(context.Background(), tenantID, id, f.ComponentID, time.Now().UTC())
			}
			if g.presenceStore != nil {
				_ = g.presenceStore.SetConnectHeartbeat(context.Background(), tenantID, id, 30*time.Second)
				_ = g.presenceStore.SetComponentHeartbeat(context.Background(), tenantID, id, f.ComponentID, 30*time.Second)
			}
			return
		}
		if f.ComponentID == "" {
			return
		}
		connectID = f.ConnectID
		if f.TenantID != "" {
			tenantID = f.TenantID
		}
		if connectID == "" {
			connectID = f.ComponentID
		}
		if err := g.registry.Register(tenantID, connectID, f.ConnectName, f.Version, f.ComponentID, f.ComponentName, f.Transport, f.UpstreamURL, r.RemoteAddr, time.Now().UTC()); err != nil {
			_ = current.write(protocol.Frame{Type: "error", ComponentID: f.ComponentID, Error: err.Error()})
			return
		}
		if g.connectStore != nil {
			if err := g.connectStore.RegisterConnect(context.Background(), store.RegisterConnectInput{TenantID: tenantID, ConnectID: connectID, ConnectName: f.ConnectName, Version: f.Version, ComponentID: f.ComponentID, ComponentName: f.ComponentName, Transport: f.Transport, UpstreamURL: f.UpstreamURL, RemoteAddr: r.RemoteAddr}, time.Now().UTC()); err != nil {
				_ = current.write(protocol.Frame{Type: "error", ComponentID: f.ComponentID, Error: err.Error()})
				return
			}
		}
		if g.presenceStore != nil {
			_ = g.presenceStore.SetConnectHeartbeat(context.Background(), tenantID, connectID, 30*time.Second)
			_ = g.presenceStore.SetComponentHeartbeat(context.Background(), tenantID, connectID, f.ComponentID, 30*time.Second)
		}
		// The first hello is handled as a registration frame. Component metadata is
		// carried by the frame and stored by the registry before routing begins.
		g.mu.Lock()
		g.sessions[routeKey(tenantID, f.ComponentID)] = s
		g.mu.Unlock()
	})
	if connectID != "" {
		g.registry.Disconnect(tenantID, connectID)
		if g.connectStore != nil {
			_ = g.connectStore.Disconnect(context.Background(), tenantID, connectID, time.Now().UTC())
		}
		if g.presenceStore != nil {
			_ = g.presenceStore.ClearConnect(context.Background(), tenantID, connectID)
		}
	}
	g.mu.Lock()
	for name, cur := range g.sessions {
		if cur == s {
			delete(g.sessions, name)
		}
	}
	g.mu.Unlock()
}
func ListenAndServe(addr string, g *Gateway) error { return http.ListenAndServe(addr, g.Handler()) }
