package connect

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type ComponentConfig struct {
	Name       string            `json:"name"`
	Transport  string            `json:"transport"`
	Command    string            `json:"command,omitempty"`
	Args       []string          `json:"args,omitempty"`
	URL        string            `json:"url,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	AllowHosts []string          `json:"allow_hosts,omitempty"`
	Enabled    bool              `json:"enabled"`
}

type Config struct {
	GatewayURL   string            `json:"gateway_url"`
	GatewayToken string            `json:"gateway_token,omitempty"`
	TenantID     string            `json:"tenant_id"`
	ConnectID    string            `json:"connect_id"`
	ConnectName  string            `json:"connect_name"`
	Version      string            `json:"version"`
	Components   []ComponentConfig `json:"components"`
}

type Store struct {
	Path string
	mu   sync.Mutex
}

func DefaultPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "mcp-connect", "config.json")
	}
	return filepath.Join(".mcp-connect", "config.json")
}

func NewStore(path string) *Store {
	if path == "" {
		path = DefaultPath()
	}
	return &Store{Path: path}
}

func (s *Store) Load() (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{GatewayURL: "ws://127.0.0.1:3080/tunnel", TenantID: "demo", Version: "0.1.0"}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	if c.GatewayURL == "" {
		c.GatewayURL = "ws://127.0.0.1:3080/tunnel"
	}
	if c.TenantID == "" {
		c.TenantID = "demo"
	}
	if c.Version == "" {
		c.Version = "0.1.0"
	}
	return c, nil
}

func (s *Store) Save(c Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}
