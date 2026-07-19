//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"zaas/api/internal/store"
)

func setupTestDB(t *testing.T) *store.PostgresStore {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("zaas_test"),
		tcpostgres.WithUsername("zaas"),
		tcpostgres.WithPassword("zaas"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	pool, err := store.ConnectAndMigrate(ctx, connStr)
	if err != nil {
		t.Fatalf("connect and migrate: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	return store.NewPostgresStore(pool)
}

func TestCreateAndGetClient(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.CreateClient(ctx, "test@example.com", "Test App")
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	client, err := s.GetClientByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("get client: %v", err)
	}
	if client.Email != "test@example.com" {
		t.Errorf("email = %q, want %q", client.Email, "test@example.com")
	}
	if client.DisplayName != "Test App" {
		t.Errorf("display_name = %q, want %q", client.DisplayName, "Test App")
	}
	if client.VerifiedAt != nil {
		t.Error("verified_at should be nil for new client")
	}
}

func TestCreateClientDuplicate(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	s := setupTestDB(t)
	ctx := context.Background()

	_ = s.CreateClient(ctx, "dup@example.com", "First")
	err := s.CreateClient(ctx, "dup@example.com", "Second")
	if !errors.Is(err, store.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestMarkClientVerified(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	s := setupTestDB(t)
	ctx := context.Background()

	_ = s.CreateClient(ctx, "verify@example.com", "Verify Me")
	err := s.MarkClientVerified(ctx, "verify@example.com", "hash123", "zaas_ab")
	if err != nil {
		t.Fatalf("mark verified: %v", err)
	}

	client, _ := s.GetClientByEmail(ctx, "verify@example.com")
	if client.VerifiedAt == nil {
		t.Error("verified_at should not be nil")
	}
	if client.APIKeyHash != "hash123" {
		t.Errorf("api_key_hash = %q, want %q", client.APIKeyHash, "hash123")
	}
}

func TestGetClientByAPIKeyHash(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	s := setupTestDB(t)
	ctx := context.Background()

	_ = s.CreateClient(ctx, "keylookup@example.com", "Key Lookup")
	_ = s.MarkClientVerified(ctx, "keylookup@example.com", "keyhash456", "zaas_kl")

	client, err := s.GetClientByAPIKeyHash(ctx, "keyhash456")
	if err != nil {
		t.Fatalf("get by key hash: %v", err)
	}
	if client.Email != "keylookup@example.com" {
		t.Errorf("email = %q, want %q", client.Email, "keylookup@example.com")
	}
}

func TestRevokeClient(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	s := setupTestDB(t)
	ctx := context.Background()

	_ = s.CreateClient(ctx, "revoke@example.com", "Revoke Me")
	err := s.RevokeClient(ctx, "revoke@example.com")
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}

	client, _ := s.GetClientByEmail(ctx, "revoke@example.com")
	if client.RevokedAt == nil {
		t.Error("revoked_at should not be nil")
	}
}

func TestCreateAndConsumeToken(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	s := setupTestDB(t)
	ctx := context.Background()

	expiresAt := time.Now().Add(24 * time.Hour)
	err := s.CreateToken(ctx, "token@example.com", "tokenhash789", store.TokenTypeRegistration, expiresAt)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	token, err := s.ConsumeToken(ctx, "tokenhash789", store.TokenTypeRegistration)
	if err != nil {
		t.Fatalf("consume token: %v", err)
	}
	if token.Email != "token@example.com" {
		t.Errorf("email = %q, want %q", token.Email, "token@example.com")
	}
	if token.UsedAt == nil {
		t.Error("used_at should not be nil after consumption")
	}
}

func TestConsumeTokenExpired(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	s := setupTestDB(t)
	ctx := context.Background()

	expiresAt := time.Now().Add(-1 * time.Hour) // already expired
	_ = s.CreateToken(ctx, "expired@example.com", "expiredhash", store.TokenTypeRegistration, expiresAt)

	_, err := s.ConsumeToken(ctx, "expiredhash", store.TokenTypeRegistration)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for expired token, got %v", err)
	}
}

func TestConsumeTokenWrongType(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	s := setupTestDB(t)
	ctx := context.Background()

	expiresAt := time.Now().Add(24 * time.Hour)
	_ = s.CreateToken(ctx, "type@example.com", "typehash", store.TokenTypeRegistration, expiresAt)

	_, err := s.ConsumeToken(ctx, "typehash", store.TokenTypeReissue)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for wrong type, got %v", err)
	}
}

func TestConsumeTokenAlreadyUsed(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	s := setupTestDB(t)
	ctx := context.Background()

	expiresAt := time.Now().Add(24 * time.Hour)
	_ = s.CreateToken(ctx, "used@example.com", "usedhash", store.TokenTypeRegistration, expiresAt)

	// First consumption succeeds
	_, _ = s.ConsumeToken(ctx, "usedhash", store.TokenTypeRegistration)

	// Second consumption fails
	_, err := s.ConsumeToken(ctx, "usedhash", store.TokenTypeRegistration)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for already-used token, got %v", err)
	}
}
