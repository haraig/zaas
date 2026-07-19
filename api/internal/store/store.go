// Package store provides the database access layer for the ZaaS API,
// wrapping pgx/pgxpool for PostgreSQL operations on client and token records.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver for database/sql (used by goose)
	"github.com/pressly/goose/v3"

	"zaas/api/internal/store/migrations"
)

// TokenType represents the purpose of a verification token.
type TokenType string

const (
	TokenTypeRegistration TokenType = "registration"
	TokenTypeReissue      TokenType = "reissue"
)

// Client represents a registered API client.
type Client struct {
	ID           string
	Email        string
	DisplayName  string
	APIKeyHash   string
	APIKeyPrefix string
	RateLimitRPM int
	CreatedAt    time.Time
	VerifiedAt   *time.Time
	RevokedAt    *time.Time
}

// Token represents a verification token.
type Token struct {
	ID        string
	Email     string
	TokenHash string
	Type      TokenType
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// ClientStore defines operations on the clients table.
type ClientStore interface {
	CreateClient(ctx context.Context, email, displayName string) error
	GetClientByEmail(ctx context.Context, email string) (*Client, error)
	GetClientByAPIKeyHash(ctx context.Context, keyHash string) (*Client, error)
	MarkClientVerified(ctx context.Context, email, apiKeyHash, apiKeyPrefix string) error
	RevokeClient(ctx context.Context, email string) error
}

// TokenStore defines operations on the verification_tokens table.
type TokenStore interface {
	CreateToken(ctx context.Context, email, tokenHash string, tokenType TokenType, expiresAt time.Time) error
	ConsumeToken(ctx context.Context, tokenHash string, tokenType TokenType) (*Token, error)
}

// BuildPoolConfig parses a database URL into a pgxpool.Config with the
// otelpgx tracer wired in so every query becomes a child span.
func BuildPoolConfig(databaseURL string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()
	return cfg, nil
}

// ConnectAndMigrate creates a pgxpool connection pool and runs pending goose migrations.
// Returns the pool (caller is responsible for calling pool.Close).
func ConnectAndMigrate(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := BuildPoolConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// Goose needs a *sql.DB. Open one using the pgx stdlib driver.
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("open sql.DB for goose: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		pool.Close()
		return nil, fmt.Errorf("set goose dialect: %w", err)
	}

	if err := goose.Up(sqlDB, "."); err != nil {
		pool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return pool, nil
}
