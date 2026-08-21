package gateway

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type localLimiter struct {
	mu      sync.Mutex
	windows map[string]rateWindow
}
type rateWindow struct {
	started time.Time
	count   int
}

func (l *localLimiter) allow(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	v := l.windows[key]
	if v.started.IsZero() || now.Sub(v.started) >= window {
		l.windows[key] = rateWindow{started: now, count: 1}
		return true
	}
	if v.count >= limit {
		return false
	}
	v.count++
	l.windows[key] = v
	return true
}

func (g *Gateway) adminAuth(next http.Handler) http.Handler {
	token := os.Getenv("MCP_ADMIN_TOKEN")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" && strings.HasPrefix(r.URL.Path, "/api/admin/") && !secureEqual(r.Header.Get("Authorization"), "Bearer "+token) {
			http.Error(w, "admin authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func secureEqual(a, b string) bool {
	x, y := []byte(a), []byte(b)
	if len(x) != len(y) {
		return false
	}
	var v byte
	for i := range x {
		v |= x[i] ^ y[i]
	}
	return v == 0
}
func bearer(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if len(v) < 7 || !strings.EqualFold(v[:7], "bearer ") {
		return ""
	}
	return strings.TrimSpace(v[7:])
}
func (g *Gateway) authenticateRoute(r *http.Request, tenant string) bool {
	if g.authStore == nil {
		return os.Getenv("MCP_REQUIRE_ROUTE_AUTH") != "true"
	}
	token := bearer(r)
	if token == "" {
		return false
	}
	id, err := g.authStore.ValidateAccessToken(r.Context(), hashTokenForGateway(token), time.Now())
	return err == nil && id.TenantID == tenant
}
func hashString(v string) string {
	h := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
func (g *Gateway) rateLimit(w http.ResponseWriter, r *http.Request, key string) bool {
	limit, _ := strconv.Atoi(os.Getenv("MCP_RATE_LIMIT"))
	if limit <= 0 {
		limit = 120
	}
	if g.presenceStore != nil {
		ok, err := g.presenceStore.Allow(r.Context(), key, limit, time.Minute)
		if err == nil && !ok {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return false
		}
		return true
	}
	if g.localLimiter == nil {
		g.localLimiter = &localLimiter{windows: map[string]rateWindow{}}
	}
	if !g.localLimiter.allow(key, limit, time.Minute) {
		http.Error(w, "rate limit exceeded", 429)
		return false
	}
	return true
}
func metricsJSON(w http.ResponseWriter, values map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(values)
}
func privateOrMetadata(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	return ip.String() == "169.254.169.254" || ip.String() == "100.100.100.200"
}
