package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zaas/api/internal/store"
)

// mockClientStore implements store.ClientStore for testing.
type mockClientStore struct {
	client *store.Client
	err    error
}

func (m *mockClientStore) GetClientByAPIKeyHash(_ context.Context, _ string) (*store.Client, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.client, nil
}

func (m *mockClientStore) CreateClient(_ context.Context, _, _ string) error { return nil }
func (m *mockClientStore) GetClientByEmail(_ context.Context, _ string) (*store.Client, error) {
	return nil, nil
}
func (m *mockClientStore) MarkClientVerified(_ context.Context, _, _, _ string) error { return nil }
func (m *mockClientStore) RevokeClient(_ context.Context, _ string) error             { return nil }

func TestAuth_NoHeader_PassesThrough(t *testing.T) {
	t.Parallel()
	mw := Auth(&mockClientStore{})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetClient(r.Context()) != nil {
			t.Fatal("expected nil client for anonymous")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuth_ValidKey_SetsContext(t *testing.T) {
	t.Parallel()
	key := "zaas_deadbeefdeadbeefdeadbeefdeadbeef"
	hash := sha256.Sum256([]byte(key))
	hashHex := hex.EncodeToString(hash[:])

	now := time.Now()
	client := &store.Client{
		ID: "client-uuid", DisplayName: "Test", RateLimitRPM: 600,
		APIKeyHash: hashHex, VerifiedAt: &now,
	}
	mw := Auth(&mockClientStore{client: client})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := GetClient(r.Context())
		if info == nil || info.ID != "client-uuid" {
			t.Fatalf("expected client in context, got %v", info)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuth_InvalidKey_Returns401(t *testing.T) {
	t.Parallel()
	mw := Auth(&mockClientStore{err: store.ErrNotFound})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	req.Header.Set("Authorization", "Bearer zaas_invalidkey00000000000000000000")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_RevokedKey_Returns401(t *testing.T) {
	t.Parallel()
	now := time.Now()
	verified := time.Now().Add(-time.Hour)
	client := &store.Client{ID: "x", RevokedAt: &now, VerifiedAt: &verified}
	mw := Auth(&mockClientStore{client: client})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	req.Header.Set("Authorization", "Bearer zaas_deadbeefdeadbeefdeadbeefdeadbeef")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
