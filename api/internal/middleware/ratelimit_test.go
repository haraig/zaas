package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zaas/api/internal/ratelimiter"
)

// mockRateLimiter records which method was called.
type mockRateLimiter struct {
	allowCalled  bool
	allowNCalled bool
	allowNRPM    int
	allowKey     string
	result       ratelimiter.Result
	err          error
}

func (m *mockRateLimiter) Allow(key string) (ratelimiter.Result, error) {
	m.allowCalled = true
	m.allowKey = key
	return m.result, m.err
}

func (m *mockRateLimiter) AllowN(key string, rpm int) (ratelimiter.Result, error) {
	m.allowNCalled = true
	m.allowNRPM = rpm
	m.allowKey = key
	return m.result, m.err
}

func (m *mockRateLimiter) Close() error { return nil }

func TestRateLimit_Anonymous_UsesAllow(t *testing.T) {
	t.Parallel()
	rl := &mockRateLimiter{result: ratelimiter.Result{Allowed: true, Remaining: 59, ResetAt: time.Now().Add(time.Minute)}}
	mw := RateLimit(rl, 60, false)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !rl.allowCalled {
		t.Fatal("expected Allow to be called for anonymous request")
	}
	if rl.allowKey != "1.2.3.4" {
		t.Fatalf("expected key '1.2.3.4', got '%s'", rl.allowKey)
	}
}

func TestRateLimit_Authenticated_UsesAllowN(t *testing.T) {
	t.Parallel()
	rl := &mockRateLimiter{result: ratelimiter.Result{Allowed: true, Remaining: 599, ResetAt: time.Now().Add(time.Minute)}}
	mw := RateLimit(rl, 60, false)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	ctx := WithClient(req.Context(), &ClientInfo{ID: "client-123", RateLimitRPM: 600})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !rl.allowNCalled {
		t.Fatal("expected AllowN to be called for authenticated request")
	}
	if rl.allowNRPM != 600 {
		t.Fatalf("expected AllowN rpm=600, got %d", rl.allowNRPM)
	}
	if rl.allowKey != "client:client-123" {
		t.Fatalf("expected key 'client:client-123', got '%s'", rl.allowKey)
	}
}

func TestRateLimit_FailOpen_OnError(t *testing.T) {
	t.Parallel()
	rl := &mockRateLimiter{err: errors.New("redis down")}
	reached := false
	mw := RateLimit(rl, 60, false)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/dice", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !reached {
		t.Fatal("fail-open: next handler should have been called on rate-limiter error")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("fail-open: expected 200, got %d", rec.Code)
	}
}

func TestRateLimit_FailClosed_OnError(t *testing.T) {
	t.Parallel()
	rl := &mockRateLimiter{err: errors.New("redis down")}
	reached := false
	mw := RateLimit(rl, 5, true)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/register", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if reached {
		t.Fatal("fail-closed: next handler must NOT be called when rate-limiter errors")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("fail-closed: expected 503, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("fail-closed: expected Retry-After header on 503")
	}
}

// --- extractIP ---

func TestExtractIP_IPv4_RemoteAddr(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	if got := extractIP(req); got != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %q", got)
	}
}

func TestExtractIP_IPv6_RemoteAddr(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	req.RemoteAddr = "[::1]:5678"
	if got := extractIP(req); got != "::1" {
		t.Errorf("expected ::1, got %q", got)
	}
}

func TestExtractIP_SingleXFF(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.5")
	if got := extractIP(req); got != "203.0.113.5" {
		t.Errorf("expected 203.0.113.5, got %q", got)
	}
}

// TestExtractIP_SpoofedFirstXFF verifies that a client-supplied first XFF entry
// is ignored in favour of the rightmost entry appended by the trusted Caddy proxy.
func TestExtractIP_SpoofedFirstXFF(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	// Attacker provides a spoofed first entry; Caddy appends the real IP at the end.
	req.Header.Set("X-Forwarded-For", "1.1.1.1, 203.0.113.99")
	if got := extractIP(req); got != "203.0.113.99" {
		t.Errorf("expected rightmost entry 203.0.113.99, got %q (spoofed first entry must not win)", got)
	}
}

func TestExtractIP_MultiHopXFF(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "5.5.5.5, 6.6.6.6, 7.7.7.7")
	if got := extractIP(req); got != "7.7.7.7" {
		t.Errorf("expected rightmost entry 7.7.7.7, got %q", got)
	}
}
