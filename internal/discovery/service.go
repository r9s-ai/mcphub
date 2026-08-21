package discovery

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
)

var (
	ErrGroupNotFound   = errors.New("group_not_found")
	ErrGroupNotAllowed = errors.New("group_not_allowed")
	ErrGroupExists     = errors.New("group_exists")
	ErrToolNotFound    = errors.New("tool_not_found")
)

type Tool struct {
	ComponentID string         `json:"component"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
	Required    []string       `json:"required,omitempty"`
	Enabled     bool           `json:"enabled"`
}
type Group struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Default     bool     `json:"is_default"`
}
type Identity struct {
	TenantID, DefaultGroup, ActiveGroup string
	AllowedGroups                       []string
}
type SearchRequest struct {
	Identity Identity
	Query    string
	Limit    int
	Cursor   string
}
type SearchResult struct {
	Group      Group  `json:"group"`
	Items      []Tool `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}
type InvokeRequest struct {
	Identity              Identity
	ComponentID, ToolName string
	Arguments             map[string]any
}
type Invoker func(context.Context, string, string, string, map[string]any) (any, error)

// Store is the persistence boundary for groups and the tool catalog. The
// in-memory maps remain the default so the Gateway can run without services.
type Store interface {
	ListGroups(context.Context, string) ([]Group, error)
	CreateGroup(context.Context, string, Group) error
	UpdateGroup(context.Context, string, Group) error
	DeleteGroup(context.Context, string, string) error
	ListGroupTools(context.Context, string, string) ([]Tool, error)
	AttachTool(context.Context, string, string, Tool) error
	DetachTool(context.Context, string, string, string, string) error
	ReplaceTools(context.Context, string, string, []Tool) error
}

type Service struct {
	mu          sync.RWMutex
	groups      map[string]map[string]Group
	tools       map[string]map[string]Tool
	memberships map[string]map[string]map[string]bool
	invoke      Invoker
	backend     Store
}

func New(invoker Invoker) *Service {
	return &Service{groups: map[string]map[string]Group{}, tools: map[string]map[string]Tool{}, memberships: map[string]map[string]map[string]bool{}, invoke: invoker}
}
func NewWithStore(invoker Invoker, backend Store) *Service {
	s := New(invoker)
	s.backend = backend
	return s
}
func (s *Service) EnsureDefault(tenant string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.groups[tenant] == nil {
		s.groups[tenant] = map[string]Group{}
	}
	if len(s.groups[tenant]) == 0 {
		s.groups[tenant]["default"] = Group{ID: "default", Name: "Default", Default: true}
	}
}
func (s *Service) UpsertGroup(tenant string, g Group) error {
	if s.backend != nil {
		if err := s.backend.CreateGroup(context.Background(), tenant, g); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.groups[tenant] == nil {
		s.groups[tenant] = map[string]Group{}
	}
	for id, existing := range s.groups[tenant] {
		if id != g.ID && strings.EqualFold(existing.Name, g.Name) {
			return ErrGroupExists
		}
	}
	s.groups[tenant][g.ID] = g
	return nil
}
func (s *Service) UpdateGroup(tenant string, g Group) error {
	if s.backend != nil {
		if err := s.backend.UpdateGroup(context.Background(), tenant, g); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[tenant][g.ID]; !ok {
		return ErrGroupNotFound
	}
	for id, existing := range s.groups[tenant] {
		if id != g.ID && strings.EqualFold(existing.Name, g.Name) {
			return ErrGroupExists
		}
	}
	s.groups[tenant][g.ID] = g
	return nil
}
func (s *Service) DeleteGroup(tenant, id string) error {
	if s.backend != nil {
		if err := s.backend.DeleteGroup(context.Background(), tenant, id); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[tenant][id]; !ok {
		return ErrGroupNotFound
	}
	delete(s.groups[tenant], id)
	if s.memberships[tenant] != nil {
		delete(s.memberships[tenant], id)
	}
	return nil
}
func (s *Service) GroupTools(tenant, group string) ([]Tool, error) {
	if s.backend != nil {
		if tools, err := s.backend.ListGroupTools(context.Background(), tenant, group); err == nil {
			s.mu.Lock()
			if s.tools[tenant] == nil {
				s.tools[tenant] = map[string]Tool{}
			}
			if s.memberships[tenant] == nil {
				s.memberships[tenant] = map[string]map[string]bool{}
			}
			if s.memberships[tenant][group] == nil {
				s.memberships[tenant][group] = map[string]bool{}
			}
			for _, t := range tools {
				key := t.ComponentID + ":" + t.Name
				s.tools[tenant][key] = t
				s.memberships[tenant][group][key] = true
			}
			s.mu.Unlock()
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.groups[tenant][group]; !ok {
		return nil, ErrGroupNotFound
	}
	out := []Tool{}
	for key := range s.memberships[tenant][group] {
		if t, ok := s.tools[tenant][key]; ok {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func (s *Service) AttachTool(tenant, group string, t Tool) error {
	if s.backend != nil {
		if err := s.backend.AttachTool(context.Background(), tenant, group, t); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[tenant][group]; !ok {
		return ErrGroupNotFound
	}
	key := t.ComponentID + ":" + t.Name
	if _, ok := s.tools[tenant][key]; !ok {
		return ErrToolNotFound
	}
	if s.memberships[tenant] == nil {
		s.memberships[tenant] = map[string]map[string]bool{}
	}
	if s.memberships[tenant][group] == nil {
		s.memberships[tenant][group] = map[string]bool{}
	}
	s.memberships[tenant][group][key] = true
	return nil
}
func (s *Service) DetachTool(tenant, group, component, tool string) error {
	if s.backend != nil {
		if err := s.backend.DetachTool(context.Background(), tenant, group, component, tool); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[tenant][group]; !ok {
		return ErrGroupNotFound
	}
	delete(s.memberships[tenant][group], component+":"+tool)
	return nil
}
func (s *Service) AddTool(tenant, group string, t Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t.Enabled = true
	if s.tools[tenant] == nil {
		s.tools[tenant] = map[string]Tool{}
	}
	key := t.ComponentID + ":" + t.Name
	s.tools[tenant][key] = t
	if s.memberships[tenant] == nil {
		s.memberships[tenant] = map[string]map[string]bool{}
	}
	if s.memberships[tenant][group] == nil {
		s.memberships[tenant][group] = map[string]bool{}
	}
	s.memberships[tenant][group][key] = true
}
func (s *Service) ReplaceTools(tenant, component string, tools []Tool) {
	if s.backend != nil {
		_ = s.backend.ReplaceTools(context.Background(), tenant, component, tools)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tools[tenant] == nil {
		s.tools[tenant] = map[string]Tool{}
	}
	if s.memberships[tenant] == nil {
		s.memberships[tenant] = map[string]map[string]bool{}
	}
	if s.memberships[tenant]["default"] == nil {
		s.memberships[tenant]["default"] = map[string]bool{}
	}
	for key := range s.tools[tenant] {
		if strings.HasPrefix(key, component+":") {
			delete(s.tools[tenant], key)
		}
	}
	for _, t := range tools {
		t.ComponentID = component
		t.Enabled = true
		key := component + ":" + t.Name
		s.tools[tenant][key] = t
		s.memberships[tenant]["default"][key] = true
	}
}
func (s *Service) group(id Identity) (Group, error) {
	g, ok := s.groups[id.TenantID][id.ActiveGroup]
	if !ok {
		return Group{}, ErrGroupNotFound
	}
	allowed := id.AllowedGroups
	if len(allowed) > 0 {
		ok = false
		for _, v := range allowed {
			if v == id.ActiveGroup {
				ok = true
			}
		}
		if !ok {
			return Group{}, ErrGroupNotAllowed
		}
	}
	return g, nil
}
func (s *Service) Search(_ context.Context, r SearchRequest) (SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, err := s.group(r.Identity)
	if err != nil {
		return SearchResult{}, err
	}
	q := strings.ToLower(strings.TrimSpace(r.Query))
	limit := r.Limit
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	out := SearchResult{Group: g, Items: []Tool{}}
	for key := range s.memberships[r.Identity.TenantID][g.ID] {
		t := s.tools[r.Identity.TenantID][key]
		if !t.Enabled {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(t.Name+" "+t.Description), q) {
			continue
		}
		t.InputSchema = nil
		out.Items = append(out.Items, t)
		if len(out.Items) >= limit {
			break
		}
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].Name < out.Items[j].Name })
	return out, nil
}
func (s *Service) Get(_ context.Context, id Identity, component, tool string) (Tool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, err := s.group(id)
	if err != nil {
		return Tool{}, err
	}
	key := component + ":" + tool
	if !s.memberships[id.TenantID][g.ID][key] {
		return Tool{}, ErrToolNotFound
	}
	t, ok := s.tools[id.TenantID][key]
	if !ok || !t.Enabled {
		return Tool{}, ErrToolNotFound
	}
	return t, nil
}
func (s *Service) SetGroup(id Identity, group string) (Identity, Group, error) {
	s.mu.RLock()
	g, ok := s.groups[id.TenantID][group]
	s.mu.RUnlock()
	if !ok {
		return id, Group{}, ErrGroupNotFound
	}
	if len(id.AllowedGroups) > 0 {
		ok = false
		for _, v := range id.AllowedGroups {
			if v == group {
				ok = true
			}
		}
		if !ok {
			return id, Group{}, ErrGroupNotAllowed
		}
	}
	id.ActiveGroup = group
	return id, g, nil
}
func (s *Service) ListGroups(tenant string) []Group {
	if s.backend != nil {
		if groups, err := s.backend.ListGroups(context.Background(), tenant); err == nil {
			s.mu.Lock()
			if s.groups[tenant] == nil {
				s.groups[tenant] = map[string]Group{}
			}
			for _, g := range groups {
				s.groups[tenant][g.ID] = g
			}
			s.mu.Unlock()
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Group, 0, len(s.groups[tenant]))
	for _, g := range s.groups[tenant] {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func (s *Service) Invoke(ctx context.Context, r InvokeRequest) (any, error) {
	if _, err := s.Get(ctx, r.Identity, r.ComponentID, r.ToolName); err != nil {
		return nil, err
	}
	if s.invoke == nil {
		return nil, errors.New("invoker_unavailable")
	}
	return s.invoke(ctx, r.Identity.TenantID, r.ComponentID, r.ToolName, r.Arguments)
}
