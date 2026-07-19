package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"zaas/api/internal/config"
	"zaas/api/internal/handler"
	"zaas/api/internal/ratelimiter"
)

func newTestRouter(cfg config.Config, rl ratelimiter.RateLimiter, promHandler http.Handler) http.Handler {
	srv := handler.New(cfg, handler.Deps{})
	return handler.NewRouter(cfg, rl, promHandler, srv, nil)
}

func TestRouterRoutes(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		CORSOrigins:  "*",
		RateLimitRPM: 60,
		BaseURL:      "http://localhost:8080",
		OTelEnabled:  false,
	}
	rl := ratelimiter.NewMemoryRateLimiter(cfg.RateLimitRPM)
	router := newTestRouter(cfg, rl, nil)

	routes := []string{
		"/api/v1/dice",
		"/api/v1/coin",
		"/api/v1/number",
		"/api/v1/uuid",
		"/api/v1/color",
		"/api/v1/coordinates",
		"/healthz",
		"/readyz",
	}

	for _, path := range routes {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code == http.StatusNotFound {
			t.Errorf("route %s returned 404 - not registered", path)
		}
	}
}

func TestRouterMetricsRoute_EnabledWithHandler(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		CORSOrigins:            "*",
		RateLimitRPM:           60,
		BaseURL:                "http://localhost:8080",
		MetricsEndpointEnabled: true,
	}
	rl := ratelimiter.NewMemoryRateLimiter(cfg.RateLimitRPM)
	stubPromHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router := newTestRouter(cfg, rl, stubPromHandler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/metrics: got %d, want 200", w.Code)
	}
}

func TestRouterMetricsRoute_DisabledByDefault(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		CORSOrigins:            "*",
		RateLimitRPM:           60,
		BaseURL:                "http://localhost:8080",
		MetricsEndpointEnabled: false,
	}
	rl := ratelimiter.NewMemoryRateLimiter(cfg.RateLimitRPM)
	router := newTestRouter(cfg, rl, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("/metrics (disabled): got %d, want 404", w.Code)
	}
}

func TestRouterOTelMiddleware_DoesNotPanic(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		CORSOrigins:  "*",
		RateLimitRPM: 60,
		BaseURL:      "http://localhost:8080",
		OTelEnabled:  false,
	}
	rl := ratelimiter.NewMemoryRateLimiter(cfg.RateLimitRPM)
	router := newTestRouter(cfg, rl, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz with otelhttp wrapper: got %d, want 200", w.Code)
	}
}
