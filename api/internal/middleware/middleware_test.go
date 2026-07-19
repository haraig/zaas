package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"zaas/api/internal/middleware"
	"zaas/api/internal/ratelimiter"
)

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func applyMiddleware(mw func(http.Handler) http.Handler) http.Handler {
	return mw(http.HandlerFunc(okHandler))
}

func TestRequestID_SetsContext(t *testing.T) {
	t.Parallel()
	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = chimiddleware.GetReqID(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	handler := chimiddleware.RequestID(inner)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if capturedID == "" {
		t.Error("request ID not set in context")
	}
}

func TestCORS_WildcardOrigin(t *testing.T) {
	t.Parallel()
	handler := applyMiddleware(middleware.CORS("*"))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("CORS origin: got %q, want *", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_SpecificOrigin(t *testing.T) {
	t.Parallel()
	handler := applyMiddleware(middleware.CORS("https://zaas.at"))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://zaas.at")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "https://zaas.at" {
		t.Errorf("CORS origin: got %q, want https://zaas.at", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestRateLimitMiddleware_SetsHeaders(t *testing.T) {
	t.Parallel()
	rl := ratelimiter.NewMemoryRateLimiter(60)
	handler := applyMiddleware(middleware.RateLimit(rl, 60, "http://localhost:8080", false))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/coin", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("X-RateLimit-Limit header not set")
	}
	if w.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("X-RateLimit-Remaining header not set")
	}
	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("X-RateLimit-Reset header not set")
	}
}

func TestRateLimitMiddleware_Blocks(t *testing.T) {
	t.Parallel()
	rl := ratelimiter.NewMemoryRateLimiter(1)
	mw := middleware.RateLimit(rl, 1, "http://localhost:8080", false)

	h := mw(http.HandlerFunc(okHandler))

	ip := "9.9.9.9:1234"
	// First request: allowed (rpm=1)
	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req1.RemoteAddr = ip
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request: got %d, want 200", w1.Code)
	}

	// Second request immediately: should be blocked
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req2.RemoteAddr = ip
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: got %d, want 429", w2.Code)
	}
	if w2.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header not set on 429")
	}
}
