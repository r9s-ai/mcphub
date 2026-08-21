package redis

import (
	"context"
	"time"

	"github.com/r9s-ai/mcphub/internal/store"
	"github.com/redis/go-redis/v9"
)

type PresenceStore struct {
	Client *redis.Client
	Prefix string
}

func New(url string) (*PresenceStore, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return &PresenceStore{Client: redis.NewClient(opt), Prefix: "mcphub"}, nil
}
func (s *PresenceStore) Close() error { return s.Client.Close() }
func connectKey(prefix, tenant, id string) string {
	return prefix + ":connect:" + tenant + ":" + id + ":heartbeat"
}
func componentKey(prefix, tenant, connect, id string) string {
	return prefix + ":component:" + tenant + ":" + connect + ":" + id + ":heartbeat"
}
func (s *PresenceStore) SetConnectHeartbeat(ctx context.Context, tenant, id string, ttl time.Duration) error {
	return s.Client.Set(ctx, connectKey(s.Prefix, tenant, id), "1", ttl).Err()
}
func (s *PresenceStore) SetComponentHeartbeat(ctx context.Context, tenant, connect, id string, ttl time.Duration) error {
	return s.Client.Set(ctx, componentKey(s.Prefix, tenant, connect, id), "1", ttl).Err()
}
func (s *PresenceStore) ClearConnect(ctx context.Context, tenant, id string) error {
	return s.Client.Del(ctx, connectKey(s.Prefix, tenant, id)).Err()
}
func (s *PresenceStore) ClearComponent(ctx context.Context, tenant, connect, id string) error {
	return s.Client.Del(ctx, componentKey(s.Prefix, tenant, connect, id)).Err()
}
func (s *PresenceStore) ConnectOnline(ctx context.Context, tenant, id string) (bool, error) {
	v, err := s.Client.Exists(ctx, connectKey(s.Prefix, tenant, id)).Result()
	return v > 0, err
}
func (s *PresenceStore) ComponentOnline(ctx context.Context, tenant, connect, id string) (bool, error) {
	v, err := s.Client.Exists(ctx, componentKey(s.Prefix, tenant, connect, id)).Result()
	return v > 0, err
}
func (s *PresenceStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	k := "mcphub:rate:" + key
	n, err := s.Client.Incr(ctx, k).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		_ = s.Client.Expire(ctx, k, window).Err()
	}
	return n <= int64(limit), nil
}
func (s *PresenceStore) MarkTokenRevoked(ctx context.Context, hash string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return s.Client.Set(ctx, "mcphub:token:revoked:"+hash, "1", ttl).Err()
}
func (s *PresenceStore) TokenRevoked(ctx context.Context, hash string) (bool, error) {
	n, err := s.Client.Exists(ctx, "mcphub:token:revoked:"+hash).Result()
	return n > 0, err
}

var _ store.PresenceStore = (*PresenceStore)(nil)
