package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/r9s-ai/mcphub/internal/connector"
	"github.com/r9s-ai/mcphub/pkg/protocol"
)

func main() {
	var gatewayURL, name, transport, command, upstream string
	var connectID, connectName, version string
	var allowHosts string
	flag.StringVar(&gatewayURL, "gateway", "ws://127.0.0.1:3080/tunnel", "gateway websocket URL")
	flag.StringVar(&name, "name", "demo", "component name")
	flag.StringVar(&transport, "transport", "streamable-http", "stdio or streamable-http")
	flag.StringVar(&command, "command", "", "stdio command")
	flag.StringVar(&upstream, "url", "http://127.0.0.1:8081/mcp", "streamable HTTP URL")
	flag.StringVar(&allowHosts, "allow-host", "", "explicitly allow upstream host(s), comma-separated")
	flag.StringVar(&connectID, "connect-id", "", "stable connection identifier")
	flag.StringVar(&connectName, "connect-name", "", "connection display name")
	flag.StringVar(&version, "version", "0.1.0", "mcp-connect version")
	flag.Parse()
	var c connector.Connector
	if transport == "stdio" {
		if command == "" {
			log.Fatal("--command is required for stdio")
		}
		fields := strings.Fields(command)
		c = connector.NewStdioConnector(connector.StdioConfig{Name: name, Command: fields[0], Args: fields[1:]})
	} else {
		var err error
		var allowed []string
		for _, host := range strings.Split(allowHosts, ",") {
			if host = strings.TrimSpace(host); host != "" {
				allowed = append(allowed, host)
			}
		}
		c, err = connector.NewHTTPConnector(connector.HTTPConfig{Name: name, URL: upstream, AllowHosts: allowed})
		if err != nil {
			log.Fatal(err)
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if connectID == "" {
		connectID = name
	}
	if connectName == "" {
		connectName = connectID
	}
	if err := c.Start(ctx); err != nil {
		log.Fatal(err)
	}
	u, err := url.Parse(gatewayURL)
	if err != nil {
		log.Fatal(err)
	}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	var writeMu sync.Mutex
	write := func(frame protocol.Frame) error { writeMu.Lock(); defer writeMu.Unlock(); return conn.WriteJSON(frame) }
	_ = write(protocol.Frame{Type: "hello", Protocol: protocol.Version, ConnectID: connectID, ConnectName: connectName, Version: version, ComponentID: name, ComponentName: name, Transport: transport, UpstreamURL: upstream})
	log.Printf("mcp-connect component %s connected", name)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				_ = write(protocol.Frame{Type: "heartbeat", ConnectID: connectID, ComponentID: name, Timestamp: now.UTC().Format(time.RFC3339)})
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
