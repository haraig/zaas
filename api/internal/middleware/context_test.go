package middleware

import (
	"context"
	"testing"
)

func TestClientContext_RoundTrip(t *testing.T) {
	t.Parallel()
	info := &ClientInfo{ID: "abc-123", DisplayName: "Test", RateLimitRPM: 600}
	ctx := WithClient(context.Background(), info)
	got := GetClient(ctx)
	if got == nil || got.ID != "abc-123" {
		t.Fatalf("expected client info, got %v", got)
	}
}

func TestClientContext_Anonymous(t *testing.T) {
	t.Parallel()
	got := GetClient(context.Background())
	if got != nil {
		t.Fatalf("expected nil for anonymous, got %v", got)
	}
}
