package registry

import (
	"testing"
	"time"
)

func TestRegistryLifecycle(t *testing.T) {
	r := New()
	now := time.Now().UTC()
	r.Register("connect-1", "laptop", "0.1.0", "tools", "tools", "streamable-http", "https://example.test/mcp", "127.0.0.1:1", now)
	connects, components := r.Snapshot()
	if len(connects) != 1 || len(components) != 1 || components[0].Status != "online" {
		t.Fatalf("unexpected snapshot: %#v %#v", connects, components)
	}
	r.Heartbeat("connect-1", "tools", now.Add(time.Second))
	r.Disconnect("connect-1")
	_, components = r.Snapshot()
	if components[0].Status != "offline" {
		t.Fatal("expected component to be offline after disconnect")
	}
	r.Heartbeat("connect-1", "tools", now.Add(2*time.Second))
	_, components = r.Snapshot()
	if components[0].Status != "online" {
		t.Fatal("expected heartbeat to restore component")
	}
	r.Expire(now.Add(heartbeatTTL + 3*time.Second))
	_, components = r.Snapshot()
	if components[0].Status != "offline" {
		t.Fatal("expected expired component to be offline")
	}
}
