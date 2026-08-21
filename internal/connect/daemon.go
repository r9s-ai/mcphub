package connect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/r9s-ai/mcphub/internal/connector"
	"github.com/r9s-ai/mcphub/pkg/protocol"
)

type Runtime struct {
	cfg       ComponentConfig
	connector connector.Connector
	cancel    context.CancelFunc
}
type Daemon struct {
	store    *Store
	cfg      Config
	mu       sync.Mutex
	runtimes map[string]*Runtime
	socket   string
	server   *http.Server
	tunnelMu sync.Mutex
	tunnel   *websocket.Conn
}

func NewDaemon(store *Store, socket string) (*Daemon, error) {
	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}
	if socket == "" {
		socket = defaultSocket()
	}
	return &Daemon{store: store, cfg: cfg, runtimes: map[string]*Runtime{}, socket: socket}, nil
}
func defaultSocket() string {
	if d, err := os.UserConfigDir(); err == nil {
		return d + "/mcp-connect/mcp-connect.sock"
	}
	return ".mcp-connect/mcp-connect.sock"
}

func (d *Daemon) Start(ctx context.Context) error {
	if err := os.MkdirAll(filepathDir(d.socket), 0700); err != nil {
		return err
	}
	_ = os.Remove(d.socket)
	l, err := net.Listen("unix", d.socket)
	if err != nil {
		return err
	}
	_ = os.Chmod(d.socket, 0600)
	d.server = &http.Server{Handler: d.handler()}
	go func() { <-ctx.Done(); d.server.Shutdown(context.Background()); _ = os.Remove(d.socket) }()
	for _, c := range d.cfg.Components {
		if c.Enabled {
			_ = d.startComponent(c)
		}
	}
	go d.runTunnel(ctx)
	err = d.server.Serve(l)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
func filepathDir(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return "."
	}
	return p[:i]
}
func (d *Daemon) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", d.status)
	mux.HandleFunc("/components", d.components)
	mux.HandleFunc("/components/", d.component)
	mux.HandleFunc("/shutdown", d.shutdown)
	return mux
}
func (d *Daemon) shutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	w.WriteHeader(202)
	go func() {
		if d.server != nil {
			_ = d.server.Shutdown(context.Background())
		}
	}()
}
func (d *Daemon) status(w http.ResponseWriter, _ *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := map[string]any{"gateway_url": d.cfg.GatewayURL, "tenant_id": d.cfg.TenantID, "connect_id": d.cfg.ConnectID, "connect_name": d.cfg.ConnectName, "components": d.cfg.Components}
	writeJSON(w, out)
}
func (d *Daemon) components(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		d.status(w, r)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	var c ComponentConfig
	if json.NewDecoder(r.Body).Decode(&c) != nil || c.Name == "" {
		http.Error(w, "invalid component", 400)
		return
	}
	c.Enabled = true
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, x := range d.cfg.Components {
		if x.Name == c.Name {
			http.Error(w, "component already exists", 409)
			return
		}
	}
	d.cfg.Components = append(d.cfg.Components, c)
	if err := d.store.Save(d.cfg); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := d.startComponentLocked(c); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, c)
}
func (d *Daemon) component(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/components/")
	if strings.HasSuffix(name, "/test") {
		name = strings.TrimSuffix(name, "/test")
		d.testComponent(w, r, name)
		return
	}
	if strings.HasSuffix(name, "/enable") {
		name = strings.TrimSuffix(name, "/enable")
		d.setEnabled(w, r, name, true)
		return
	}
	if strings.HasSuffix(name, "/disable") {
		name = strings.TrimSuffix(name, "/disable")
		d.setEnabled(w, r, name, false)
		return
	}
	if r.Method != "DELETE" {
		w.WriteHeader(405)
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	found := false
	next := d.cfg.Components[:0]
	for _, c := range d.cfg.Components {
		if c.Name == name {
			found = true
			d.stopComponentLocked(name)
			continue
		}
		next = append(next, c)
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	d.cfg.Components = next
	_ = d.store.Save(d.cfg)
	w.WriteHeader(204)
}
func (d *Daemon) testComponent(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	d.mu.Lock()
	runtime := d.runtimes[name]
	d.mu.Unlock()
	if runtime == nil {
		http.Error(w, "component is disabled or not running", 503)
		return
	}
	if err := runtime.connector.Health(r.Context()); err != nil {
		writeJSON(w, map[string]any{"name": name, "healthy": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"name": name, "healthy": true})
}
func (d *Daemon) setEnabled(w http.ResponseWriter, r *http.Request, name string, enabled bool) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	found := false
	for i := range d.cfg.Components {
		if d.cfg.Components[i].Name == name {
			found = true
			d.cfg.Components[i].Enabled = enabled
			if enabled {
				if err := d.startComponentLocked(d.cfg.Components[i]); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
			} else {
				d.stopComponentLocked(name)
			}
			break
		}
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	if err := d.store.Save(d.cfg); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"name": name, "enabled": enabled})
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func (d *Daemon) startComponent(c ComponentConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.startComponentLocked(c)
}
func (d *Daemon) startComponentLocked(c ComponentConfig) error {
	if _, ok := d.runtimes[c.Name]; ok {
		return nil
	}
	var co connector.Connector
	var err error
	if c.Transport == "stdio" {
		if c.Command == "" {
			return fmt.Errorf("command is required")
		}
		co = connector.NewStdioConnector(connector.StdioConfig{Name: c.Name, Command: c.Command, Args: c.Args})
	} else {
		co, err = connector.NewHTTPConnector(connector.HTTPConfig{Name: c.Name, URL: c.URL, Headers: c.Headers, AllowHosts: c.AllowHosts})
		if err != nil {
			return err
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := co.Start(ctx); err != nil {
		cancel()
		return err
	}
	r := &Runtime{cfg: c, connector: co, cancel: cancel}
	d.runtimes[c.Name] = r
	d.tunnelMu.Lock()
	if d.tunnel != nil {
		_ = d.tunnel.Close()
	}
	d.tunnelMu.Unlock()
	return nil
}
func (d *Daemon) stopComponentLocked(name string) {
	if r := d.runtimes[name]; r != nil {
		r.cancel()
		_ = r.connector.Stop(context.Background())
		delete(d.runtimes, name)
		d.tunnelMu.Lock()
		if d.tunnel != nil {
			_ = d.tunnel.Close()
		}
		d.tunnelMu.Unlock()
	}
}
func (d *Daemon) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for n := range d.runtimes {
		d.stopComponentLocked(n)
	}
}

func (d *Daemon) runTunnel(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		u, err := url.Parse(d.cfg.GatewayURL)
		if err == nil {
			var conn *websocket.Conn
			conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
			if err == nil {
				d.tunnelMu.Lock()
				d.tunnel = conn
				d.tunnelMu.Unlock()
				backoff = time.Second
				err = d.serveTunnel(ctx, conn)
				_ = conn.Close()
				d.tunnelMu.Lock()
				if d.tunnel == conn {
					d.tunnel = nil
				}
				d.tunnelMu.Unlock()
			}
		}
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}
func (d *Daemon) serveTunnel(ctx context.Context, conn *websocket.Conn) error {
	var mu sync.Mutex
	write := func(f protocol.Frame) error { mu.Lock(); defer mu.Unlock(); return conn.WriteJSON(f) }
	d.mu.Lock()
	runtimes := make(map[string]*Runtime, len(d.runtimes))
	for name, runtime := range d.runtimes {
		runtimes[name] = runtime
	}
	d.mu.Unlock()
	for _, r := range runtimes {
		if err := write(protocol.Frame{Type: "hello", Protocol: protocol.Version, TenantID: d.cfg.TenantID, GatewayToken: d.cfg.GatewayToken, ConnectID: d.cfg.ConnectID, ConnectName: d.cfg.ConnectName, Version: d.cfg.Version, ComponentID: r.cfg.Name, ComponentName: r.cfg.Name, Transport: r.cfg.Transport, UpstreamURL: r.cfg.URL}); err != nil {
			return err
		}
	}
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				d.mu.Lock()
				names := make([]string, 0, len(d.runtimes))
				for name := range d.runtimes {
					names = append(names, name)
				}
				d.mu.Unlock()
				for _, name := range names {
					_ = write(protocol.Frame{Type: "heartbeat", TenantID: d.cfg.TenantID, ConnectID: d.cfg.ConnectID, ComponentID: name, Timestamp: now.UTC().Format(time.RFC3339)})
				}
			}
		}
	}()
	active := map[string]context.CancelFunc{}
	var activeMu sync.Mutex
	for {
		_, b, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var f protocol.Frame
		if json.Unmarshal(b, &f) != nil {
			continue
		}
		if f.Type == "cancel" {
			activeMu.Lock()
			if cancel := active[f.StreamID]; cancel != nil {
				cancel()
				delete(active, f.StreamID)
			}
			activeMu.Unlock()
			continue
		}
		if f.Type == "error" {
			return fmt.Errorf("gateway registration failed: %s", f.Error)
		}
		if f.Type != "request" {
			continue
		}
		d.mu.Lock()
		r := d.runtimes[f.ComponentID]
		d.mu.Unlock()
		if r == nil {
			_ = write(protocol.Frame{Type: "response", StreamID: f.StreamID, Status: 404, Error: "component not found", EndOfStream: true})
			continue
		}
		go func(f protocol.Frame) {
			requestCtx, cancel := context.WithCancel(ctx)
			activeMu.Lock()
			active[f.StreamID] = cancel
			activeMu.Unlock()
			defer func() { cancel(); activeMu.Lock(); delete(active, f.StreamID); activeMu.Unlock() }()
			emit := func(res *connector.MCPResponse) error {
				return write(protocol.Frame{Type: "response", StreamID: f.StreamID, Status: res.Status, Headers: res.Headers, Payload: res.Payload, EndOfStream: res.EndOfStream})
			}
			var e error
			if stream, ok := r.connector.(connector.StreamConnector); ok {
				e = stream.HandleStream(requestCtx, &connector.MCPRequest{Method: f.Method, Headers: f.Headers, Payload: f.Payload}, emit)
			} else {
				var res *connector.MCPResponse
				res, e = r.connector.Handle(requestCtx, &connector.MCPRequest{Method: f.Method, Headers: f.Headers, Payload: f.Payload})
				if e == nil {
					e = emit(res)
				}
			}
			if e != nil {
				_ = write(protocol.Frame{Type: "response", StreamID: f.StreamID, Status: 502, Error: e.Error(), EndOfStream: true})
			}
		}(f)
	}
}

// DialControl connects to the local daemon socket and performs one request.
func DialControl(socket, method, path string, body any) ([]byte, int, error) {
	if socket == "" {
		socket = defaultSocket()
	}
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) { return net.Dial("unix", socket) }}
	client := &http.Client{Transport: tr}
	var rd *strings.Reader
	if body == nil {
		rd = strings.NewReader("")
	} else {
		b, _ := json.Marshal(body)
		rd = strings.NewReader(string(b))
	}
	req, err := http.NewRequest(method, "http://unix"+path, rd)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	b, err := ioReadAll(res.Body)
	return b, res.StatusCode, err
}
func ioReadAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) { return io.ReadAll(r) }
