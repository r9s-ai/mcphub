package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var req protocol.Frame
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
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
