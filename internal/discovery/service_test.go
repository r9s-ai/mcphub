package discovery

import (
	"context"
	"testing"
)

func TestGroupScopedDiscoveryAndTenantIsolation(t *testing.T) {
	var calledTenant string
	s := New(func(_ context.Context, tenant, component, tool string, _ map[string]any) (any, error) {
		calledTenant = tenant
		return map[string]any{"component": component, "tool": tool}, nil
	})
	s.EnsureDefault("a")
	s.UpsertGroup("a", Group{ID: "ops", Name: "Ops"})
	s.EnsureDefault("b")
	s.AddTool("a", "default", Tool{ComponentID: "github", Name: "create_issue", Description: "Create issue"})
	s.AddTool("a", "ops", Tool{ComponentID: "k8s", Name: "pods"})

	id := Identity{TenantID: "a", DefaultGroup: "default", ActiveGroup: "default", AllowedGroups: []string{"default", "ops"}}
	res, err := s.Search(context.Background(), SearchRequest{Identity: id, Query: "issue"})
	if err != nil || len(res.Items) != 1 || res.Items[0].ComponentID != "github" {
		t.Fatalf("unexpected search: %+v %v", res, err)
	}
	if _, _, err := s.SetGroup(id, "missing"); err != ErrGroupNotFound {
		t.Fatalf("expected group_not_found, got %v", err)
	}
	if _, _, err := s.SetGroup(id, "ops"); err != nil {
		t.Fatal(err)
	}
	opsID, _, _ := s.SetGroup(id, "ops")
	if _, err := s.Get(context.Background(), opsID, "github", "create_issue"); err != ErrToolNotFound {
		t.Fatalf("cross-group get allowed: %v", err)
	}
	if _, err := s.Invoke(context.Background(), InvokeRequest{Identity: Identity{TenantID: "a", ActiveGroup: "ops", AllowedGroups: []string{"ops"}}, ComponentID: "k8s", ToolName: "pods"}); err != nil {
		t.Fatal(err)
	}
	if calledTenant != "a" {
		t.Fatalf("invoker tenant=%q", calledTenant)
	}
	if _, err := s.Search(context.Background(), SearchRequest{Identity: Identity{TenantID: "b", ActiveGroup: "default"}}); err != nil {
		t.Fatal(err)
	}
}

func TestGroupToolManagement(t *testing.T) {
	s := New(nil)
	s.EnsureDefault("demo")
	s.UpsertGroup("demo", Group{ID: "g", Name: "G"})
	s.AddTool("demo", "default", Tool{ComponentID: "c", Name: "t"})
	if err := s.AttachTool("demo", "g", Tool{ComponentID: "c", Name: "t"}); err != nil {
		t.Fatal(err)
	}
	tools, err := s.GroupTools("demo", "g")
	if err != nil || len(tools) != 1 {
		t.Fatalf("tools=%v err=%v", tools, err)
	}
	if err := s.DetachTool("demo", "g", "c", "t"); err != nil {
		t.Fatal(err)
	}
	if tools, _ := s.GroupTools("demo", "g"); len(tools) != 0 {
		t.Fatal("tool not detached")
	}
	if err := s.DeleteGroup("demo", "g"); err != nil {
		t.Fatal(err)
	}
}
