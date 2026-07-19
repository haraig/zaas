package store_test

import (
	"testing"

	"zaas/api/internal/store"
)

// TestBuildPoolConfigSetsOTelTracer verifies that the pool configuration
// built from a DSN has the otelpgx tracer wired in.
func TestBuildPoolConfigSetsOTelTracer(t *testing.T) {
	t.Parallel()
	cfg, err := store.BuildPoolConfig("postgres://zaas:zaas@localhost:5432/zaas_test?sslmode=disable")
	if err != nil {
		t.Fatalf("BuildPoolConfig: %v", err)
	}
	if cfg.ConnConfig.Tracer == nil {
		t.Error("pool config ConnConfig.Tracer must not be nil after BuildPoolConfig")
	}
}
