package eventstoreui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthenticatorSessionLifecycle(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	auth := newAuthenticator(Config{
		Username: "developer", Password: "secret", SessionSecret: []byte(strings.Repeat("x", 32)), SessionTTL: time.Hour,
	})
	auth.now = func() time.Time { return now }

	if !auth.validCredentials("developer", "secret") || auth.validCredentials("developer", "wrong") {
		t.Fatal("credential validation returned an unexpected result")
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "http://example.test/api/v1/auth/login", nil)
	if err := auth.setSession(recorder, request); err != nil {
		t.Fatal(err)
	}
	response := recorder.Result()
	cookies := response.Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName || !cookies[0].HttpOnly {
		t.Fatalf("unexpected session cookie: %#v", cookies)
	}

	authenticatedRequest := httptest.NewRequest(http.MethodGet, "http://example.test/api/v1/stores", nil)
	authenticatedRequest.AddCookie(cookies[0])
	if !auth.authenticated(authenticatedRequest) {
		t.Fatal("expected signed cookie to authenticate")
	}
	auth.now = func() time.Time { return now.Add(2 * time.Hour) }
	if auth.authenticated(authenticatedRequest) {
		t.Fatal("expected expired cookie to be rejected")
	}
}

func TestRequireSameOrigin(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "https://events.example.test/api/v1/auth/login", nil)
	request.Header.Set("Origin", "https://events.example.test")
	if err := requireSameOrigin(request); err != nil {
		t.Fatalf("expected same origin to be accepted: %v", err)
	}
	request.Header.Set("Origin", "https://attacker.example")
	if err := requireSameOrigin(request); err == nil {
		t.Fatal("expected foreign origin to be rejected")
	}
}

func TestAuthenticatorAllowsRequestsWhenDisabled(t *testing.T) {
	auth := newAuthenticator(Config{})
	request := httptest.NewRequest(http.MethodGet, "http://example.test/api/v1/stores", nil)

	if auth.enabled() {
		t.Fatal("expected authentication to be disabled")
	}
	if !auth.authenticated(request) {
		t.Fatal("expected requests to be allowed when authentication is disabled")
	}
	if auth.validCredentials("", "") {
		t.Fatal("empty credentials must not be treated as a valid login")
	}
}
