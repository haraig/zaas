//go:build integration

package ratelimiter_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"zaas/api/internal/ratelimiter"
)

func startRedis(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	container, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("terminate redis container: %v", err)
		}
	})
	url, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("get redis connection string: %v", err)
	}
	return url
}

func TestRedisRateLimiter_AllowsWithinLimit(t *testing.T) {
	t.Parallel()
	url := startRedis(t)
	rl, err := ratelimiter.NewRedisRateLimiter(url, 10)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}
	defer rl.Close()

	for i := 0; i < 10; i++ {
		result, err := rl.Allow("ip:1.2.3.4")
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if !result.Allowed {
			t.Fatalf("request %d: expected allowed, got blocked (remaining=%d)", i, result.Remaining)
		}
	}
}

func TestRedisRateLimiter_BlocksAtLimit(t *testing.T) {
	t.Parallel()
	url := startRedis(t)
	rl, err := ratelimiter.NewRedisRateLimiter(url, 5)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}
	defer rl.Close()

	for i := 0; i < 5; i++ {
		if _, err := rl.Allow("ip:10.0.0.1"); err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
	}
	result, err := rl.Allow("ip:10.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected blocked after limit reached")
	}
	if result.Remaining != 0 {
		t.Errorf("expected remaining=0 when blocked, got %d", result.Remaining)
	}
}

func TestRedisRateLimiter_DifferentKeysAreIsolated(t *testing.T) {
	t.Parallel()
	url := startRedis(t)
	rl, err := ratelimiter.NewRedisRateLimiter(url, 2)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}
	defer rl.Close()

	for i := 0; i < 2; i++ {
		if _, err := rl.Allow("ip:1.1.1.1"); err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
	}
	blocked, err := rl.Allow("ip:1.1.1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked.Allowed {
		t.Error("ip:1.1.1.1 should be blocked")
	}

	allowed, err := rl.Allow("ip:2.2.2.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed.Allowed {
		t.Error("ip:2.2.2.2 should be allowed (different key)")
	}
}

func TestRedisRateLimiter_AllowN_DynamicRPM(t *testing.T) {
	t.Parallel()
	url := startRedis(t)
	rl, err := ratelimiter.NewRedisRateLimiter(url, 60)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}
	defer rl.Close()

	// Use AllowN with rpm=3 - only 3 requests should be allowed.
	for i := 0; i < 3; i++ {
		result, err := rl.AllowN("client:abc", 3)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if !result.Allowed {
			t.Fatalf("request %d: expected allowed", i)
		}
	}
	result, err := rl.AllowN("client:abc", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("4th request should be blocked with rpm=3")
	}
}

func TestRedisRateLimiter_ResultFields(t *testing.T) {
	t.Parallel()
	url := startRedis(t)
	rl, err := ratelimiter.NewRedisRateLimiter(url, 10)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}
	defer rl.Close()

	result, err := rl.Allow("ip:3.3.3.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("first request should be allowed")
	}
	if result.Remaining < 0 {
		t.Errorf("remaining should be >= 0, got %d", result.Remaining)
	}
	if result.ResetAt.IsZero() {
		t.Error("resetAt should not be zero")
	}
}

func TestRedisRateLimiter_RemainingDecrementsCorrectly(t *testing.T) {
	t.Parallel()
	url := startRedis(t)
	rl, err := ratelimiter.NewRedisRateLimiter(url, 5)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}
	defer rl.Close()

	for i := 0; i < 5; i++ {
		result, err := rl.Allow("ip:4.4.4.4")
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		expected := 5 - (i + 1)
		if result.Remaining != expected {
			t.Errorf("request %d: expected remaining=%d, got %d", i, expected, result.Remaining)
		}
	}
}

func TestRedisRateLimiter_AllowN_ZeroRPM(t *testing.T) {
	t.Parallel()
	url := startRedis(t)
	rl, err := ratelimiter.NewRedisRateLimiter(url, 60)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}
	defer rl.Close()

	result, err := rl.AllowN("ip:5.5.5.5", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("rpm=0 should deny all requests")
	}
	if result.Remaining != 0 {
		t.Errorf("expected remaining=0 for rpm=0, got %d", result.Remaining)
	}
}

// TestRedisRateLimiter_EmitsOTelSpan verifies that the Redis client is instrumented
// with redisotel so that Allow calls produce child spans visible to the OTel pipeline.
func TestRedisRateLimiter_EmitsOTelSpan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Start the container BEFORE setting up the OTel provider so that testcontainers
	// bootstrap spans do not contaminate the exporter.
	url := startRedis(t)

	// Set up an in-memory span recorder as the global provider.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(old)

	// Create the rate limiter after setting up the provider so that
	// redisotel.InstrumentTracing picks up our test provider.
	rl, err := ratelimiter.NewRedisRateLimiter(url, 10)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}
	defer rl.Close()

	spansBefore := len(exporter.GetSpans())

	_, err = rl.Allow("ip:1.2.3.4")
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}

	spansAfter := len(exporter.GetSpans())
	if spansAfter <= spansBefore {
		t.Errorf("expected Redis OTel spans after Allow call; spans before=%d, after=%d", spansBefore, spansAfter)
	}
}
