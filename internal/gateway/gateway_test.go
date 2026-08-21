package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/r9s-ai/mcphub/pkg/protocol"
)

func TestGatewayForwardsStreamingFrames(t *testing.T) {
	g := New()
	ts := httptest.NewServer(g.Handler())
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/tunnel"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(protocol.Frame{Type: "hello", Protocol: protocol.Version, ConnectID: "c1", ConnectName: "test", ComponentID: "stream", ComponentName: "stream", Transport: "streamable-http"}); err != nil {
		t.Fatal(err)
	}
	result := make(chan *http.Response, 1)
	go func() {
		res, err := http.Post(ts.URL+"/mcp/demo/stream", "application/json", strings.NewReader(`{"jsonrpc":"2.0"}`))
		if err != nil {
			result <- nil
			return
		}
		result <- res
	}()
	var req protocol.Frame
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Fatal(err)
		}
		var rpc struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(req.Payload, &rpc)
		if rpc.Method == "tools/list" {
			if err := conn.WriteJSON(protocol.Frame{Type: "response", StreamID: req.StreamID, Status: 200, Payload: []byte(`{"jsonrpc":"2.0","id":"catalog","result":{"tools":[]}}`), EndOfStream: true}); err != nil {
				t.Fatal(err)
			}
			continue
		}
		break
	}
	if req.Type != "request" {
		t.Fatalf("expected request, got %s", req.Type)
	}
	if err := conn.WriteJSON(protocol.Frame{Type: "response", StreamID: req.StreamID, Status: 200, Headers: map[string]string{"Content-Type": "text/event-stream"}, Payload: []byte("data: one\n\n")}); err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(protocol.Frame{Type: "response", StreamID: req.StreamID, Payload: []byte("data: two\n\n"), EndOfStream: true}); err != nil {
		t.Fatal(err)
	}
	res := <-result
	if res == nil {
		t.Fatal("request failed")
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || string(body) != "data: one\n\ndata: two\n\n" {
		t.Fatalf("unexpected response: %d %q", res.StatusCode, body)
	}
}

func TestGatewayRejectsInvalidConnectToken(t *testing.T) {
	g := NewWithToken("expected")
	ts := httptest.NewServer(g.Handler())
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/tunnel"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(protocol.Frame{Type: "hello", Protocol: protocol.Version, GatewayToken: "wrong", TenantID: "demo", ConnectID: "c1", ComponentID: "tools", ComponentName: "tools"}); err != nil {
		t.Fatal(err)
	}
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var frame protocol.Frame
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatal(err)
	}
	if frame.Type != "error" {
		t.Fatalf("expected error frame, got %q", frame.Type)
	}
}

func TestGatewayServesLandingAndAdminDeepLinks(t *testing.T) {
	webDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(webDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<title>MCPHub</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "assets", "app.js"), []byte("console.log('mcphub')"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := New()
	g.webDir = webDir
	handler := g.Handler()
	for _, path := range []string{"/", "/admin", "/admin/components/example"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "MCPHub") {
			t.Fatalf("expected web app for %s, got %d: %s", path, response.Code, response.Body.String())
		}
		if response.Header().Get("Content-Security-Policy") == "" || response.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("missing web security headers for %s", path)
		}
	}
	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if asset.Code != http.StatusOK || !strings.Contains(asset.Body.String(), "mcphub") || !strings.Contains(asset.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("unexpected asset response: %d %s", asset.Code, asset.Body.String())
	}
	unknownAPI := httptest.NewRecorder()
	handler.ServeHTTP(unknownAPI, httptest.NewRequest(http.MethodGet, "/api/not-a-route", nil))
	if unknownAPI.Code != http.StatusNotFound || strings.Contains(unknownAPI.Body.String(), "MCPHub") {
		t.Fatalf("unknown API route must not be handled by SPA: %d %s", unknownAPI.Code, unknownAPI.Body.String())
	}
}
