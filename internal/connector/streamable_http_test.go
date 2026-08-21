package connector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPConnectorProxy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Mcp-Session-Id") != "s1" {
			t.Errorf("session header not forwarded")
		}
		w.Header().Set("Mcp-Session-Id", "s2")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("data: ok\n\n"))
	}))
	defer ts.Close()
	c, err := NewHTTPConnector(HTTPConfig{Name: "x", URL: ts.URL, AllowHosts: []string{"127.0.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := c.Handle(context.Background(), &MCPRequest{Payload: []byte(`{"jsonrpc":"2.0"}`), Headers: map[string]string{"Mcp-Session-Id": "s1"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != 200 || string(r.Payload) != "data: ok\n\n" || r.Headers["Mcp-Session-Id"] != "s2" {
		t.Fatalf("bad response: %#v", r)
	}
}

func TestHTTPConnectorHandleStreamEmitsChunks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: one\n\n"))
		f.Flush()
		time.Sleep(5 * time.Millisecond)
		_, _ = w.Write([]byte("data: two\n\n"))
		f.Flush()
	}))
	defer ts.Close()
	c, err := NewHTTPConnector(HTTPConfig{Name: "stream", URL: ts.URL, AllowHosts: []string{"127.0.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	err = c.HandleStream(context.Background(), &MCPRequest{Payload: []byte(`{"jsonrpc":"2.0"}`)}, func(r *MCPResponse) error { count++; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if count < 3 {
		t.Fatalf("expected chunks and end marker, got %d", count)
	}
}

func TestHTTPConnectorRejectsUntrustedHost(t *testing.T) {
	if _, err := NewHTTPConnector(HTTPConfig{Name: "x", URL: "http://example.com/mcp"}); err == nil {
		t.Fatal("expected untrusted host to be rejected")
	}
}
