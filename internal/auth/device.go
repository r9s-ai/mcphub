package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/r9s-ai/mcphub/internal/store"
)

type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}
type deviceEntry struct {
	DeviceCode     string
	UserCode       string
	ExpiresAt      time.Time
	Approved       bool
	AccessToken    string
	TokenExpiresAt time.Time
	RefreshToken   string
	Revoked        bool
}
type DeviceAuth struct {
	mu         sync.Mutex
	entries    map[string]*deviceEntry
	publicURL  string
	backend    store.AuthStore
	revocation store.PresenceStore
}

func NewDeviceAuth(publicURL string) *DeviceAuth {
	return &DeviceAuth{entries: map[string]*deviceEntry{}, publicURL: strings.TrimRight(publicURL, "/")}
}
func NewDeviceAuthWithStore(publicURL string, backend store.AuthStore) *DeviceAuth {
	a := NewDeviceAuth(publicURL)
	a.backend = backend
	return a
}
func NewDeviceAuthWithStores(publicURL string, backend store.AuthStore, revocation store.PresenceStore) *DeviceAuth {
	a := NewDeviceAuthWithStore(publicURL, backend)
	a.revocation = revocation
	return a
}
func hashValue(v string) string {
	h := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func (a *DeviceAuth) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/device/code", a.code)
	mux.HandleFunc("/api/auth/device/token", a.token)
	mux.HandleFunc("/api/auth/token", a.oauthToken)
	mux.HandleFunc("/api/auth/revoke", a.revoke)
	mux.HandleFunc("/api/auth/device/approve", a.approve)
	mux.HandleFunc("/device", a.page)
}
func (a *DeviceAuth) code(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	e := &deviceEntry{DeviceCode: randomString(24), UserCode: strings.ToUpper(randomString(5)), ExpiresAt: time.Now().Add(10 * time.Minute)}
	if a.backend != nil {
		var in struct {
			TenantID string `json:"tenant_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.TenantID == "" {
			in.TenantID = "demo"
		}
		if err := a.backend.CreateDeviceCode(r.Context(), store.DeviceCodeInput{DeviceCodeHash: hashValue(e.DeviceCode), UserCodeHash: hashValue(e.UserCode), TenantID: in.TenantID, ExpiresAt: e.ExpiresAt}); err != nil {
			writeStatus(w, 500, "storage_error")
			return
		}
	}
	a.mu.Lock()
	a.entries[e.DeviceCode] = e
	a.mu.Unlock()
	write(w, DeviceCode{DeviceCode: e.DeviceCode, UserCode: e.UserCode, VerificationURI: a.publicURL + "/device?code=" + e.UserCode, ExpiresIn: 600, Interval: 5})
}
func (a *DeviceAuth) token(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	var in struct {
		DeviceCode string `json:"device_code"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if a.backend != nil {
		pair, err := a.backend.ExchangeDeviceCode(r.Context(), hashValue(in.DeviceCode), time.Now())
		if err != nil {
			writeStatus(w, 400, err.Error())
			return
		}
		write(w, map[string]any{"access_token": pair.AccessToken, "token_type": "Bearer", "expires_in": int(time.Until(pair.AccessExpiresAt).Seconds()), "refresh_token": pair.RefreshToken})
		return
	}
	a.mu.Lock()
	e := a.entries[in.DeviceCode]
	if e == nil || time.Now().After(e.ExpiresAt) || e.Revoked {
		a.mu.Unlock()
		writeStatus(w, 400, "expired_token")
		return
	}
	if !e.Approved {
		a.mu.Unlock()
		writeStatus(w, 428, "authorization_pending")
		return
	}
	if e.AccessToken == "" {
		e.AccessToken = randomString(32)
		e.TokenExpiresAt = time.Now().Add(time.Hour)
	}
	if e.RefreshToken == "" {
		e.RefreshToken = randomString(40)
	}
	token := e.AccessToken
	refreshToken := e.RefreshToken
	a.mu.Unlock()
	write(w, map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": 3600, "refresh_token": refreshToken})
}

func (a *DeviceAuth) oauthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeStatus(w, 400, "invalid_request")
		return
	}
	if r.FormValue("grant_type") != "refresh_token" {
		writeStatus(w, 400, "unsupported_grant_type")
		return
	}
	refresh := r.FormValue("refresh_token")
	if a.backend != nil {
		pair, err := a.backend.RefreshToken(r.Context(), hashValue(refresh), time.Now())
		if err != nil {
			writeStatus(w, 400, err.Error())
			return
		}
		write(w, map[string]any{"access_token": pair.AccessToken, "token_type": "Bearer", "expires_in": int(time.Until(pair.AccessExpiresAt).Seconds()), "refresh_token": pair.RefreshToken})
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, e := range a.entries {
		if e.RefreshToken == refresh && !e.Revoked && time.Now().Before(e.ExpiresAt) {
			e.AccessToken = randomString(32)
			e.TokenExpiresAt = time.Now().Add(time.Hour)
			write(w, map[string]any{"access_token": e.AccessToken, "token_type": "Bearer", "expires_in": 3600, "refresh_token": e.RefreshToken})
			return
		}
	}
	writeStatus(w, 400, "invalid_grant")
}
func (a *DeviceAuth) revoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	_ = r.ParseForm()
	token := r.FormValue("token")
	if a.backend != nil {
		_ = a.backend.RevokeToken(r.Context(), hashValue(token), time.Now())
		if a.revocation != nil {
			_ = a.revocation.MarkTokenRevoked(r.Context(), hashValue(token), time.Hour)
		}
		write(w, map[string]any{"revoked": true})
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, e := range a.entries {
		if e.AccessToken == token || e.RefreshToken == token {
			e.Revoked = true
			break
		}
	}
	write(w, map[string]any{"revoked": true})
}

func (a *DeviceAuth) Validate(token string) bool {
	if token == "" {
		return false
	}
	if a.backend != nil {
		if a.revocation != nil {
			revoked, err := a.revocation.TokenRevoked(context.Background(), hashValue(token))
			if err == nil && revoked {
				return false
			}
		}
		_, err := a.backend.ValidateAccessToken(context.Background(), hashValue(token), time.Now())
		return err == nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, e := range a.entries {
		if e.AccessToken == token && !e.Revoked && time.Now().Before(e.TokenExpiresAt) {
			return true
		}
	}
	return false
}
func (a *DeviceAuth) approve(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	code := r.FormValue("user_code")
	if code == "" {
		var in struct {
			UserCode string `json:"user_code"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		code = in.UserCode
	}
	if a.backend != nil {
		if err := a.backend.ApproveDeviceCode(r.Context(), hashValue(code), time.Now()); err != nil {
			writeStatus(w, 404, "invalid_user_code")
			return
		}
		write(w, map[string]any{"approved": true})
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, e := range a.entries {
		if e.UserCode == strings.ToUpper(code) && time.Now().Before(e.ExpiresAt) && !e.Revoked {
			e.Approved = true
			write(w, map[string]any{"approved": true})
			return
		}
	}
	writeStatus(w, 404, "invalid_user_code")
}
func (a *DeviceAuth) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	code := r.URL.Query().Get("code")
	fmt.Fprintf(w, `<!doctype html><title>MCPHub Device Login</title><h1>MCPHub Device Login</h1><form method="post" action="/api/auth/device/approve"><input name="user_code" value="%s" placeholder="User code"><button>Approve</button></form>`, code)
}
func write(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func writeStatus(w http.ResponseWriter, status int, code string) {
	w.WriteHeader(status)
	write(w, map[string]string{"error": code})
}
