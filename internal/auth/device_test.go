package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDeviceAuthorizationFlow(t *testing.T) {
	a := NewDeviceAuth("http://gateway")
	mux := http.NewServeMux()
	a.Register(mux)
	r := httptest.NewRecorder()
	mux.ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/auth/device/code", strings.NewReader(`{}`)))
	if r.Code != 200 {
		t.Fatalf("code status: %d", r.Code)
	}
	var code DeviceCode
	if err := json.Unmarshal(r.Body.Bytes(), &code); err != nil {
		t.Fatal(err)
	}
	r = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/device/token", strings.NewReader(`{"device_code":"`+code.DeviceCode+`"}`))
	mux.ServeHTTP(r, req)
	if r.Code != 428 {
		t.Fatalf("expected pending, got %d", r.Code)
	}
	r = httptest.NewRecorder()
	form := url.Values{"user_code": {code.UserCode}}
	req = httptest.NewRequest(http.MethodPost, "/api/auth/device/approve", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(r, req)
	if r.Code != 200 {
		t.Fatalf("approve status: %d", r.Code)
	}
	r = httptest.NewRecorder()
	mux.ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/auth/device/token", strings.NewReader(`{"device_code":"`+code.DeviceCode+`"}`)))
	if r.Code != 200 {
		t.Fatalf("token status: %d", r.Code)
	}
	var token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &token); err != nil || !a.Validate(token.AccessToken) {
		t.Fatal("issued token was not valid")
	}
	r = httptest.NewRecorder()
	refresh := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {token.RefreshToken}}
	req = httptest.NewRequest(http.MethodPost, "/api/auth/token", strings.NewReader(refresh.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(r, req)
	if r.Code != 200 {
		t.Fatalf("refresh status: %d", r.Code)
	}
	r = httptest.NewRecorder()
	revoke := url.Values{"token": {token.RefreshToken}}
	req = httptest.NewRequest(http.MethodPost, "/api/auth/revoke", strings.NewReader(revoke.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(r, req)
	if r.Code != 200 || a.Validate(token.AccessToken) {
		t.Fatal("revoked token remained valid")
	}
}
