package registry

import (
	"testing"
	"time"
)

func TestRegistryLifecycle(t *testing.T) {
	r := New()
	now := time.Now().UTC()
	r.Register("demo", "connect-1", "laptop", "0.1.0", "tools", "tools", "streamable-http", "https://example.test/mcp", "127.0.0.1:1", now)
	connects, components := r.Snapshot()
	if len(connects) != 1 || len(components) != 1 || components[0].Status != "online" {
		t.Fatalf("unexpected snapshot: %#v %#v", connects, components)
	}
	r.Heartbeat("demo", "connect-1", "tools", now.Add(time.Second))
	r.Disconnect("demo", "connect-1")
	_, components = r.Snapshot()
	if components[0].Status != "offline" {
		t.Fatal("expected component to be offline after disconnect")
	}
	r.Heartbeat("demo", "connect-1", "tools", now.Add(2*time.Second))
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

func TestRegistryRejectsDuplicateComponentNameInTenant(t *testing.T) {
	r := New()
	now := time.Now().UTC()
	if err := r.Register("demo", "connect-1", "one", "0.1.0", "tools", "tools", "stdio", "", "", now); err != nil {
		t.Fatal(err)
	}
	if err := r.Register("demo", "connect-2", "two", "0.1.0", "other", "tools", "stdio", "", "", now); err != ErrComponentNameTaken {
		t.Fatalf("expected duplicate name error, got %v", err)
	}
	if err := r.Register("other", "connect-2", "two", "0.1.0", "other", "tools", "stdio", "", "", now); err != nil {
		t.Fatal(err)
	}
}
