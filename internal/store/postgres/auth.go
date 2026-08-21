package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/r9s-ai/mcphub/internal/store"
)

func (s *ConnectStore) RecordAudit(ctx context.Context, e store.AuditEvent, at time.Time) error {
	meta := e.Metadata
	if len(meta) == 0 {
		meta = []byte(`{}`)
	}
	if !json.Valid(meta) {
		meta = []byte(`{}`)
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor,connect_id,component_id,transport,method,status,latency_ms,error_code,metadata,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, e.TenantID, e.Actor, e.ConnectID, e.ComponentID, e.Transport, e.Method, e.Status, e.Latency.Milliseconds(), e.ErrorCode, meta, at)
	return err
}

var _ store.AuditStore = (*ConnectStore)(nil)

var ErrAuthInvalid = errors.New("invalid or expired authorization")

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func (s *ConnectStore) CreateDeviceCode(ctx context.Context, in store.DeviceCodeInput) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO device_codes(device_code_hash,user_code_hash,tenant_id,expires_at) VALUES($1,$2,$3,$4)`, in.DeviceCodeHash, in.UserCodeHash, in.TenantID, in.ExpiresAt)
	return err
}
func (s *ConnectStore) ApproveDeviceCode(ctx context.Context, userHash string, at time.Time) error {
	r, err := s.Pool.Exec(ctx, `UPDATE device_codes SET approved_at=$2 WHERE user_code_hash=$1 AND approved_at IS NULL AND expires_at>$2`, userHash, at)
	if err != nil {
		return err
	}
	if r.RowsAffected() == 0 {
		return ErrAuthInvalid
	}
	return nil
}
func (s *ConnectStore) ExchangeDeviceCode(ctx context.Context, deviceHash string, at time.Time) (store.TokenPair, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return store.TokenPair{}, err
	}
	defer tx.Rollback(ctx)
	var tenant string
	var approved, consumed *time.Time
	var expires time.Time
	if err := tx.QueryRow(ctx, `SELECT tenant_id,expires_at,approved_at,consumed_at FROM device_codes WHERE device_code_hash=$1 FOR UPDATE`, deviceHash).Scan(&tenant, &expires, &approved, &consumed); err != nil || approved == nil || consumed != nil || expires.Before(at) {
		return store.TokenPair{}, ErrAuthInvalid
	}
	access, refresh := randomToken(32), randomToken(40)
	if _, err := tx.Exec(ctx, `UPDATE device_codes SET consumed_at=$2 WHERE device_code_hash=$1`, deviceHash, at); err != nil {
		return store.TokenPair{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO auth_tokens(token_hash,token_type,tenant_id,expires_at) VALUES($1,'access',$2,$3),($4,'refresh',$2,$5)`, hashToken(access), tenant, at.Add(time.Hour), hashToken(refresh), at.Add(30*24*time.Hour)); err != nil {
		return store.TokenPair{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.TokenPair{}, err
	}
	return store.TokenPair{AccessToken: access, RefreshToken: refresh, AccessExpiresAt: at.Add(time.Hour), RefreshExpiresAt: at.Add(30 * 24 * time.Hour)}, nil
}
func (s *ConnectStore) RefreshToken(ctx context.Context, refreshHash string, at time.Time) (store.TokenPair, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return store.TokenPair{}, err
	}
	defer tx.Rollback(ctx)
	var tenant, connect string
	var expires time.Time
	if err := tx.QueryRow(ctx, `SELECT tenant_id,connect_id,expires_at FROM auth_tokens WHERE token_hash=$1 AND token_type='refresh' AND revoked_at IS NULL FOR UPDATE`, refreshHash).Scan(&tenant, &connect, &expires); err != nil || expires.Before(at) {
		return store.TokenPair{}, ErrAuthInvalid
	}
	access, refresh := randomToken(32), randomToken(40)
	if _, err := tx.Exec(ctx, `UPDATE auth_tokens SET revoked_at=$2,replaced_by=$3 WHERE token_hash=$1`, refreshHash, at, hashToken(refresh)); err != nil {
		return store.TokenPair{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO auth_tokens(token_hash,token_type,tenant_id,connect_id,expires_at) VALUES($1,'access',$2,$3,$4),($5,'refresh',$2,$3,$6)`, hashToken(access), tenant, connect, at.Add(time.Hour), hashToken(refresh), at.Add(30*24*time.Hour)); err != nil {
		return store.TokenPair{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.TokenPair{}, err
	}
	return store.TokenPair{AccessToken: access, RefreshToken: refresh, AccessExpiresAt: at.Add(time.Hour), RefreshExpiresAt: at.Add(30 * 24 * time.Hour)}, nil
}
func (s *ConnectStore) RevokeToken(ctx context.Context, tokenHash string, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `UPDATE auth_tokens SET revoked_at=COALESCE(revoked_at,$2) WHERE token_hash=$1`, tokenHash, at)
	return err
}
func (s *ConnectStore) ValidateAccessToken(ctx context.Context, tokenHash string, at time.Time) (store.TokenIdentity, error) {
	var id store.TokenIdentity
	var expires time.Time
	var revoked *time.Time
	err := s.Pool.QueryRow(ctx, `SELECT tenant_id,connect_id,expires_at,revoked_at FROM auth_tokens WHERE token_hash=$1 AND token_type='access'`, tokenHash).Scan(&id.TenantID, &id.ConnectID, &expires, &revoked)
	if err != nil || revoked != nil || expires.Before(at) {
		return store.TokenIdentity{}, ErrAuthInvalid
	}
	return id, nil
}
func hashToken(v string) string {
	b := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(b[:])
}

var _ store.AuthStore = (*ConnectStore)(nil)
