package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/r9s-ai/mcphub/internal/admin"
	"github.com/r9s-ai/mcphub/internal/registry"
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
func (s *session) request(ctx context.Context, f protocol.Frame) (protocol.Frame, error) {
	ch := make(chan protocol.Frame, 1)
	s.mu.Lock()
	s.pending[f.StreamID] = ch
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.pending, f.StreamID); s.mu.Unlock() }()
	if err := s.write(f); err != nil {
		return protocol.Frame{}, err
	}
	select {
	case r := <-ch:
		return r, nil
	case <-ctx.Done():
		return protocol.Frame{}, ctx.Err()
	case <-s.closed:
		return protocol.Frame{}, context.Canceled
	}
}
func (s *session) readLoop(onHello func(protocol.Frame, *session)) {
	defer close(s.closed)
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
			ch <- f
		}
	}
}

type Gateway struct {
	mu       sync.RWMutex
	sessions map[string]*session
	registry *registry.Registry
	upgrader websocket.Upgrader
}

func New() *Gateway {
	g := &Gateway{sessions: map[string]*session{}, registry: registry.New(), upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for now := range ticker.C {
			g.registry.Expire(now)
		}
	}()
	return g
}
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp/", g.handleMCP)
	mux.HandleFunc("/tunnel", g.handleTunnel)
	admin.New(g.registry).Register(mux)
	return mux
}
func (g *Gateway) handleMCP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "mcp" {
		http.NotFound(w, r)
		return
	}
	component := parts[2]
	g.mu.RLock()
	s, ok := g.sessions[component]
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
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	id := time.Now().Format("20060102150405.000000000")
	res, err := s.request(ctx, protocol.Frame{Type: "request", StreamID: id, ComponentID: component, Method: r.Method, Headers: h, Payload: body})
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	for k, v := range res.Headers {
		w.Header().Set(k, v)
	}
	if res.Status == 0 {
		res.Status = 502
	}
	w.WriteHeader(res.Status)
	_, _ = w.Write(res.Payload)
}

func (g *Gateway) handleTunnel(w http.ResponseWriter, r *http.Request) {
	conn, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s := newSession(conn)
	defer conn.Close()
	connectID := ""
	s.readLoop(func(f protocol.Frame, current *session) {
		if f.Type == "heartbeat" {
			id := f.ConnectID
			if id == "" {
				id = connectID
			}
			g.registry.Heartbeat(id, f.ComponentID, time.Now().UTC())
			return
		}
		if f.ComponentID == "" {
			return
		}
		connectID = f.ConnectID
		if connectID == "" {
			connectID = f.ComponentID
		}
		g.registry.Register(connectID, f.ConnectName, f.Version, f.ComponentID, f.ComponentName, f.Transport, f.UpstreamURL, r.RemoteAddr, time.Now().UTC())
		// The first hello is handled as a registration frame. Component metadata is
		// carried by the frame and stored by the registry before routing begins.
		g.mu.Lock()
		g.sessions[f.ComponentID] = s
		g.mu.Unlock()
	})
	if connectID != "" {
		g.registry.Disconnect(connectID)
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
