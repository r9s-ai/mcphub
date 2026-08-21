package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/r9s-ai/mcphub/internal/connect"
	"github.com/r9s-ai/mcphub/internal/connector"
	"github.com/r9s-ai/mcphub/pkg/protocol"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		legacy(args)
		return
	}
	switch args[0] {
	case "login":
		login(args[1:])
	case "install-service":
		installService(args[1:])
	case "uninstall-service":
		uninstallService()
	case "daemon", "start":
		daemon(args[1:])
	case "add":
		add(args[1:])
	case "remove":
		remove(args[1:])
	case "test":
		componentAction("test", args[1:])
	case "enable", "disable":
		componentAction(args[0], args[1:])
	case "list", "status":
		controlArgs(args[0], args[1:], "GET", "/status")
	case "stop":
		controlArgs("stop", args[1:], "POST", "/shutdown")
	default:
		usage()
	}
}

func usage() {
	fmt.Println(`mcp-connect commands:
  mcp-connect login [--gateway URL] [--token TOKEN]
  mcp-connect install-service [--config PATH] [--socket PATH]
  mcp-connect uninstall-service
  mcp-connect daemon [--gateway URL] [--connect-id ID] [--connect-name NAME]
  mcp-connect add NAME --transport stdio --command COMMAND
  mcp-connect add NAME --transport streamable-http --url URL [--allow-host HOST]
	  mcp-connect list | status | remove NAME | stop
  mcp-connect test NAME | enable NAME | disable NAME

Legacy direct mode remains available with --gateway, --name, --transport, --url, and --command.`)
}

func installService(args []string) {
	fs := flag.NewFlagSet("install-service", flag.ExitOnError)
	config := fs.String("config", "", "configuration file")
	socket := fs.String("socket", "", "local Unix socket")
	start := fs.Bool("start", false, "enable and start the service on Linux")
	_ = fs.Parse(args)
	store := connect.NewStore(*config)
	path, err := connect.InstallService(store.Path, *socket)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("installed service file: %s\n", path)
	if *start {
		if err := connect.EnableService(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("service enabled and started")
	}
}
func uninstallService() {
	path, err := connect.UninstallService()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("removed service file: %s\n", path)
}

func login(args []string) {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	gateway := fs.String("gateway", "ws://127.0.0.1:3080/tunnel", "gateway websocket URL")
	token := fs.String("token", "", "Gateway Connect token (optional in development mode)")
	tenant := fs.String("tenant-id", "demo", "tenant identifier")
	id := fs.String("connect-id", "", "stable connection identifier")
	name := fs.String("connect-name", "", "connection display name")
	config := fs.String("config", "", "configuration file")
	socket := fs.String("socket", "", "local Unix socket")
	foreground := fs.Bool("foreground", false, "run daemon in the foreground")
	_ = fs.Parse(args)
	store := connect.NewStore(*config)
	c, err := store.Load()
	if err != nil {
		log.Fatal(err)
	}
	c.GatewayURL, c.TenantID = normalizeGatewayURL(*gateway), *tenant
	if *token != "" {
		c.GatewayToken = *token
	} else if c.GatewayToken == "" {
		c.GatewayToken = deviceLogin(*gateway)
	}
	if *id != "" {
		c.ConnectID = *id
	}
	if c.ConnectID == "" {
		c.ConnectID = "local"
	}
	if *name != "" {
		c.ConnectName = *name
	}
	if c.ConnectName == "" {
		c.ConnectName = c.ConnectID
	}
	if err := store.Save(c); err != nil {
		log.Fatal(err)
	}
	if *foreground {
		daemon([]string{"--config", store.Path, "--socket", *socket})
		return
	}
	if _, status, err := connect.DialControl(*socket, "GET", "/status", nil); err != nil || status >= 300 {
		exe, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		cmd := exec.Command(exe, "daemon", "--config", store.Path)
		if *socket != "" {
			cmd.Args = append(cmd.Args, "--socket", *socket)
		}
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}
		log.Printf("started mcp-connect daemon (pid %d)", cmd.Process.Pid)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if body, status, err := connect.DialControl(*socket, "GET", "/status", nil); err == nil && status < 300 {
			fmt.Printf("login complete; daemon is online\n%s", body)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.Fatal("daemon did not become ready within 5 seconds")
}

func deviceLogin(gateway string) string {
	base := strings.TrimSuffix(strings.TrimSuffix(gateway, "/tunnel"), "/")
	base = strings.Replace(base, "wss://", "https://", 1)
	base = strings.Replace(base, "ws://", "http://", 1)
	res, err := http.Post(base+"/api/auth/device/code", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		log.Fatalf("device login request failed: %v", err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		log.Fatalf("device login request failed: %s", b)
	}
	var code struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		Interval        int    `json:"interval"`
	}
	if err := json.Unmarshal(b, &code); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Open %s and approve code %s\n", code.VerificationURI, code.UserCode)
	interval := time.Duration(code.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for i := 0; i < 120; i++ {
		time.Sleep(interval)
		req, _ := http.NewRequest(http.MethodPost, base+"/api/auth/device/token", bytes.NewReader([]byte(fmt.Sprintf(`{"device_code":%q}`, code.DeviceCode))))
		req.Header.Set("Content-Type", "application/json")
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode == 428 {
			continue
		}
		if r.StatusCode >= 300 {
			log.Fatalf("device authorization failed: %s", body)
		}
		var token struct {
			AccessToken string `json:"access_token"`
		}
		if json.Unmarshal(body, &token) == nil && token.AccessToken != "" {
			return token.AccessToken
		}
	}
	log.Fatal("device authorization timed out")
	return ""
}

func normalizeGatewayURL(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "http://") {
		return "ws://" + strings.TrimPrefix(value, "http://")
	}
	if strings.HasPrefix(value, "https://") {
		return "wss://" + strings.TrimPrefix(value, "https://")
	}
	return value
}

func daemon(args []string) {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	gateway := fs.String("gateway", "", "gateway websocket URL")
	id := fs.String("connect-id", "", "stable connection identifier")
	tenant := fs.String("tenant-id", "", "tenant identifier")
	name := fs.String("connect-name", "", "connection display name")
	socket := fs.String("socket", "", "local Unix socket")
	config := fs.String("config", "", "configuration file")
	_ = fs.Parse(args)
	st := connect.NewStore(*config)
	c, err := st.Load()
	if err != nil {
		log.Fatal(err)
	}
	if *gateway != "" {
		c.GatewayURL = *gateway
	}
	if *id != "" {
		c.ConnectID = *id
	}
	if *name != "" {
		c.ConnectName = *name
	}
	if *tenant != "" {
		c.TenantID = *tenant
	}
	if c.ConnectID == "" {
		c.ConnectID = "local"
	}
	if c.ConnectName == "" {
		c.ConnectName = c.ConnectID
	}
	if err := st.Save(c); err != nil {
		log.Fatal(err)
	}
	d, err := connect.NewDaemon(st, *socket)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	log.Printf("mcp-connect daemon listening on %s", *socket)
	if err := d.Start(ctx); err != nil {
		log.Fatal(err)
	}
}
func add(args []string) {
	if len(args) < 1 {
		usage()
		return
	}
	name := args[0]
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	transport := fs.String("transport", "streamable-http", "stdio or streamable-http")
	command := fs.String("command", "", "stdio command")
	upstream := fs.String("url", "", "streamable HTTP URL")
	allow := fs.String("allow-host", "", "allowed upstream hosts")
	config := fs.String("config", "", "configuration file")
	gateway := fs.String("gateway", "", "gateway websocket URL")
	_ = fs.Parse(args[1:])
	st := connect.NewStore(*config)
	c, err := st.Load()
	if err != nil {
		log.Fatal(err)
	}
	if *gateway != "" {
		c.GatewayURL = *gateway
	}
	if c.ConnectID == "" {
		c.ConnectID = name
	}
	if c.ConnectName == "" {
		c.ConnectName = c.ConnectID
	}
	cc := connect.ComponentConfig{Name: name, Transport: *transport, URL: *upstream, Enabled: true}
	if *command != "" {
		f := strings.Fields(*command)
		cc.Command = f[0]
		cc.Args = f[1:]
	}
	for _, h := range strings.Split(*allow, ",") {
		if strings.TrimSpace(h) != "" {
			cc.AllowHosts = append(cc.AllowHosts, strings.TrimSpace(h))
		}
	}
	b, _ := json.Marshal(cc)
	body, status, err := connect.DialControl("", "POST", "/components", json.RawMessage(b))
	if err != nil {
		if err := st.Save(func() connect.Config { c.Components = append(c.Components, cc); return c }()); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("saved %s; start daemon to activate it\n", name)
		return
	}
	if status >= 300 {
		log.Fatalf("add failed (%d): %s", status, body)
	}
	fmt.Printf("added %s\n", name)
}
func remove(args []string) {
	if len(args) < 1 {
		usage()
		return
	}
	body, status, err := connect.DialControl("", "DELETE", "/components/"+url.PathEscape(args[0]), nil)
	if err != nil {
		log.Fatal(err)
	}
	if status >= 300 {
		log.Fatalf("remove failed (%d): %s", status, body)
	}
	fmt.Printf("removed %s\n", args[0])
}
func componentAction(action string, args []string) {
	if len(args) < 1 {
		usage()
		return
	}
	method, path := "POST", "/components/"+url.PathEscape(args[0])+"/"+action
	body, status, err := connect.DialControl("", method, path, nil)
	if err != nil {
		log.Fatalf("%s failed: %v (is mcp-connect daemon running?)", action, err)
	}
	if status >= 300 {
		log.Fatalf("%s failed (%d): %s", action, status, body)
	}
	fmt.Println(string(body))
}
func control(label, method, path string, body any) {
	controlSocket(label, "", method, path, body)
}
func controlArgs(label string, args []string, method, path string) {
	fs := flag.NewFlagSet(label, flag.ExitOnError)
	socket := fs.String("socket", "", "local Unix socket")
	_ = fs.Parse(args)
	controlSocket(label, *socket, method, path, nil)
}
func controlSocket(label, socket, method, path string, body any) {
	b, status, err := connect.DialControl(socket, method, path, body)
	if err != nil {
		log.Fatalf("%s failed: %v (is mcp-connect daemon running?)", label, err)
	}
	if status >= 300 {
		log.Fatalf("%s failed (%d): %s", label, status, b)
	}
	fmt.Println(string(b))
}

func legacy(args []string) {
	fs := flag.NewFlagSet("mcp-connect", flag.ExitOnError)
	gatewayURL := fs.String("gateway", "ws://127.0.0.1:3080/tunnel", "gateway websocket URL")
	name := fs.String("name", "demo", "component name")
	transport := fs.String("transport", "streamable-http", "stdio or streamable-http")
	command := fs.String("command", "", "stdio command")
	upstream := fs.String("url", "http://127.0.0.1:8081/mcp", "streamable HTTP URL")
	allowHosts := fs.String("allow-host", "", "allowed upstream hosts")
	connectID := fs.String("connect-id", "", "stable connection identifier")
	connectName := fs.String("connect-name", "", "connection display name")
	version := fs.String("version", "0.1.0", "mcp-connect version")
	tenantID := fs.String("tenant-id", "demo", "tenant identifier")
	token := fs.String("token", "", "Gateway Connect token")
	fs.Parse(args)
	var c connector.Connector
	var err error
	if *transport == "stdio" {
		if *command == "" {
			log.Fatal("--command is required for stdio")
		}
		f := strings.Fields(*command)
		c = connector.NewStdioConnector(connector.StdioConfig{Name: *name, Command: f[0], Args: f[1:]})
	} else {
		var allowed []string
		for _, h := range strings.Split(*allowHosts, ",") {
			if strings.TrimSpace(h) != "" {
				allowed = append(allowed, strings.TrimSpace(h))
			}
		}
		c, err = connector.NewHTTPConnector(connector.HTTPConfig{Name: *name, URL: *upstream, AllowHosts: allowed})
		if err != nil {
			log.Fatal(err)
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if *connectID == "" {
		*connectID = *name
	}
	if *connectName == "" {
		*connectName = *connectID
	}
	if err := c.Start(ctx); err != nil {
		log.Fatal(err)
	}
	u, err := url.Parse(*gatewayURL)
	if err != nil {
		log.Fatal(err)
	}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	var writeMu sync.Mutex
	write := func(f protocol.Frame) error { writeMu.Lock(); defer writeMu.Unlock(); return conn.WriteJSON(f) }
	_ = write(protocol.Frame{Type: "hello", Protocol: protocol.Version, TenantID: *tenantID, GatewayToken: *token, ConnectID: *connectID, ConnectName: *connectName, Version: *version, ComponentID: *name, ComponentName: *name, Transport: *transport, UpstreamURL: *upstream})
	log.Printf("mcp-connect component %s connected", *name)
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				_ = write(protocol.Frame{Type: "heartbeat", ConnectID: *connectID, ComponentID: *name, Timestamp: now.UTC().Format(time.RFC3339)})
			}
		}
	}()
	for {
		_, b, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var f protocol.Frame
		if json.Unmarshal(b, &f) != nil || f.Type != "request" {
			continue
		}
		res, e := c.Handle(ctx, &connector.MCPRequest{Method: f.Method, Headers: f.Headers, Payload: f.Payload})
		out := protocol.Frame{Type: "response", StreamID: f.StreamID, Status: 502, EndOfStream: true}
		if e != nil {
			out.Error = e.Error()
		} else {
			out.Status = res.Status
			out.Headers = res.Headers
			out.Payload = res.Payload
			out.EndOfStream = res.EndOfStream
		}
		if err := write(out); err != nil {
			return
		}
	}
}
