package middleware_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"zaas/api/internal/middleware"
)

func TestLogger_SkipsHealthPaths(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/healthz", "/readyz"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))
			handler := middleware.Logger(logger)(http.HandlerFunc(okHandler))

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
			req.RemoteAddr = "1.2.3.4:1234"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if buf.Len() != 0 {
				t.Errorf("expected no log output for %s, got: %s", path, buf.String())
			}
		})
	}
}

func TestLogger_LogsRegularPaths(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	handler := middleware.Logger(logger)(http.HandlerFunc(okHandler))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/dice", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if buf.Len() == 0 {
		t.Error("expected log output for /api/v1/dice, got none")
	}
}
