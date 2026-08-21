package store

import (
	"context"
	"time"

	"github.com/r9s-ai/mcphub/internal/registry"
)

type RegisterConnectInput struct {
	TenantID, ConnectID, ConnectName, Version                      string
	ComponentID, ComponentName, Transport, UpstreamURL, RemoteAddr string
}

type ConnectStore interface {
	RegisterConnect(context.Context, RegisterConnectInput, time.Time) error
	Heartbeat(context.Context, string, string, string, time.Time) error
	Disconnect(context.Context, string, string, time.Time) error
	ListConnects(context.Context, string) ([]registry.ConnectInstance, error)
	ListComponents(context.Context, string) ([]registry.RegisteredComponent, error)
	GetComponent(context.Context, string, string) (registry.RegisteredComponent, error)
}

type TokenPair struct {
	AccessToken, RefreshToken         string
	AccessExpiresAt, RefreshExpiresAt time.Time
}
type TokenIdentity struct {
	TenantID, ConnectID string
	DefaultGroupID      string
	AllowedGroupIDs     []string
}
type DeviceCodeInput struct {
	DeviceCodeHash, UserCodeHash, TenantID string
	ExpiresAt                              time.Time
}

type AuthStore interface {
	CreateDeviceCode(context.Context, DeviceCodeInput) error
	ApproveDeviceCode(context.Context, string, time.Time) error
	ExchangeDeviceCode(context.Context, string, time.Time) (TokenPair, error)
	RefreshToken(context.Context, string, time.Time) (TokenPair, error)
	RevokeToken(context.Context, string, time.Time) error
	ValidateAccessToken(context.Context, string, time.Time) (TokenIdentity, error)
}

// GroupAuthStore extends AuthStore with discovery group authorization.
type GroupAuthStore interface {
	AuthStore
	TokenGroups(context.Context, string) (TokenIdentity, error)
}

type TokenGroupStore interface {
	SetTokenGroups(context.Context, string, string, []string) error
	GetTokenGroups(context.Context, string, string) (TokenIdentity, error)
}

type PresenceStore interface {
	SetConnectHeartbeat(context.Context, string, string, time.Duration) error
	SetComponentHeartbeat(context.Context, string, string, string, time.Duration) error
	ClearConnect(context.Context, string, string) error
	ClearComponent(context.Context, string, string, string) error
	ConnectOnline(context.Context, string, string) (bool, error)
	ComponentOnline(context.Context, string, string, string) (bool, error)
	Allow(context.Context, string, int, time.Duration) (bool, error)
	MarkTokenRevoked(context.Context, string, time.Duration) error
	TokenRevoked(context.Context, string) (bool, error)
}

type CatalogCache interface {
	GetCatalog(context.Context, string, string) ([]byte, error)
	SetCatalog(context.Context, string, string, []byte, time.Duration) error
	TryCatalogLock(context.Context, string, string, time.Duration) (bool, error)
}

type AuditEvent struct {
	TenantID, Actor, ConnectID, ComponentID, Transport, Method string
	Status                                                     int
	Latency                                                    time.Duration
	ErrorCode                                                  string
	Metadata                                                   []byte
}
type AuditStore interface {
	RecordAudit(context.Context, AuditEvent, time.Time) error
}
