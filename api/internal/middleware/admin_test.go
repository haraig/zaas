package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminAuth_MissingHeader_Returns401(t *testing.T) {
	t.Parallel()
	mw := AdminAuth("secret-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAdminAuth_WrongToken_Returns401(t *testing.T) {
	t.Parallel()
	mw := AdminAuth("secret-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/", nil)
	req.Header.Set("X-Admin-Token", "wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAdminAuth_CorrectToken_PassesThrough(t *testing.T) {
	t.Parallel()
	mw := AdminAuth("secret-token")
	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/", nil)
	req.Header.Set("X-Admin-Token", "secret-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAdminAuth_EmptyConfiguredToken_AlwaysRejects(t *testing.T) {
	t.Parallel()
	mw := AdminAuth("")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/", nil)
	req.Header.Set("X-Admin-Token", "")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (fail-closed on unconfigured token), got %d", rec.Code)
	}
}
