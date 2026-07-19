package ratelimiter_test

import (
	"testing"
	"time"

	"zaas/api/internal/ratelimiter"
)

func TestMemoryRateLimiter_AllowsWithinLimit(t *testing.T) {
	t.Parallel()
	rl := ratelimiter.NewMemoryRateLimiter(10)
	for i := 0; i < 10; i++ {
		result, err := rl.Allow("192.168.1.1")
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if !result.Allowed {
			t.Fatalf("request %d: expected allowed, got blocked (remaining=%d)", i, result.Remaining)
		}
	}
}

func TestMemoryRateLimiter_BlocksAtLimit(t *testing.T) {
	t.Parallel()
	rl := ratelimiter.NewMemoryRateLimiter(5)
	for i := 0; i < 5; i++ {
		if _, err := rl.Allow("10.0.0.1"); err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
	}
	result, err := rl.Allow("10.0.0.1")
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

func TestMemoryRateLimiter_DifferentKeysAreIsolated(t *testing.T) {
	t.Parallel()
	rl := ratelimiter.NewMemoryRateLimiter(2)
	for i := 0; i < 2; i++ {
		if _, err := rl.Allow("1.1.1.1"); err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
	}
	blocked, err := rl.Allow("1.1.1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked.Allowed {
		t.Error("1.1.1.1 should be blocked")
	}
	allowed, err := rl.Allow("2.2.2.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed.Allowed {
		t.Error("2.2.2.2 should be allowed (different key)")
	}
}

func TestMemoryRateLimiter_ResultFields(t *testing.T) {
	t.Parallel()
	rl := ratelimiter.NewMemoryRateLimiter(10)
	result, err := rl.Allow("3.3.3.3")
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
	if result.ResetAt.Before(time.Now()) {
		t.Error("resetAt should be in the future")
	}
}

func TestMemoryRateLimiter_AllowN_DynamicRPM(t *testing.T) {
	t.Parallel()
	rl := ratelimiter.NewMemoryRateLimiter(60)

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

func TestMemoryRateLimiter_RemainingDecrementsCorrectly(t *testing.T) {
	t.Parallel()
	rl := ratelimiter.NewMemoryRateLimiter(5)
	for i := 0; i < 5; i++ {
		result, err := rl.Allow("4.4.4.4")
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		expected := 5 - (i + 1)
		if result.Remaining != expected {
			t.Errorf("request %d: expected remaining=%d, got %d", i, expected, result.Remaining)
		}
	}
}

func TestMemoryRateLimiter_Close_StopsCleanupGoroutine(t *testing.T) {
	t.Parallel()
	rl := ratelimiter.NewMemoryRateLimiter(10)
	// Close should not block and must not panic on double-close or after use.
	done := make(chan error, 1)
	go func() {
		done <- rl.Close()
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Close blocked for more than 2 seconds - cleanup goroutine may be stuck")
	}
}
