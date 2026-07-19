package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a query finds no rows.
var ErrNotFound = errors.New("not found")

// ErrAlreadyExists is returned when a unique constraint is violated.
var ErrAlreadyExists = errors.New("already exists")

// PostgresStore implements both ClientStore and TokenStore.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new store backed by the given connection pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// ── ClientStore ─────────────────────────────────────────────────────────────

func (s *PostgresStore) CreateClient(ctx context.Context, email, displayName string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO clients (email, display_name) VALUES ($1, $2)`,
		email, displayName,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("create client: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetClientByEmail(ctx context.Context, email string) (*Client, error) {
	return s.scanClient(ctx,
		`SELECT id, email, display_name, api_key_hash, api_key_prefix, rate_limit_rpm,
		        created_at, verified_at, revoked_at
		 FROM clients WHERE email = $1`, email)
}

func (s *PostgresStore) GetClientByAPIKeyHash(ctx context.Context, keyHash string) (*Client, error) {
	return s.scanClient(ctx,
		`SELECT id, email, display_name, api_key_hash, api_key_prefix, rate_limit_rpm,
		        created_at, verified_at, revoked_at
		 FROM clients WHERE api_key_hash = $1`, keyHash)
}

func (s *PostgresStore) MarkClientVerified(ctx context.Context, email, apiKeyHash, apiKeyPrefix string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE clients SET verified_at = now(), api_key_hash = $2, api_key_prefix = $3
		 WHERE email = $1 AND verified_at IS NULL`,
		email, apiKeyHash, apiKeyPrefix,
	)
	if err != nil {
		return fmt.Errorf("mark client verified: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) RevokeClient(ctx context.Context, email string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE clients SET revoked_at = now() WHERE email = $1 AND revoked_at IS NULL`,
		email,
	)
	if err != nil {
		return fmt.Errorf("revoke client: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) scanClient(ctx context.Context, query string, args ...any) (*Client, error) {
	var c Client
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&c.ID, &c.Email, &c.DisplayName, &c.APIKeyHash, &c.APIKeyPrefix,
		&c.RateLimitRPM, &c.CreatedAt, &c.VerifiedAt, &c.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan client: %w", err)
	}
	return &c, nil
}

// ── TokenStore ──────────────────────────────────────────────────────────────

func (s *PostgresStore) CreateToken(ctx context.Context, email, tokenHash string, tokenType TokenType, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO verification_tokens (email, token_hash, type, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		email, tokenHash, string(tokenType), expiresAt,
	)
	if err != nil {
		return fmt.Errorf("create token: %w", err)
	}
	return nil
}

func (s *PostgresStore) ConsumeToken(ctx context.Context, tokenHash string, tokenType TokenType) (*Token, error) {
	var t Token
	var typStr string
	err := s.pool.QueryRow(ctx,
		`UPDATE verification_tokens
		 SET used_at = now()
		 WHERE token_hash = $1 AND type = $2 AND used_at IS NULL AND expires_at > now()
		 RETURNING id, email, token_hash, type, created_at, expires_at, used_at`,
		tokenHash, string(tokenType),
	).Scan(&t.ID, &t.Email, &t.TokenHash, &typStr, &t.CreatedAt, &t.ExpiresAt, &t.UsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("consume token: %w", err)
	}
	t.Type = TokenType(typStr)
	return &t, nil
}

// isDuplicateKeyError checks if the error is a PostgreSQL unique violation (SQLSTATE 23505).
func isDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
