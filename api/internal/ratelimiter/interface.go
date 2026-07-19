// Package ratelimiter defines the RateLimiter interface and provides a
// memory-backed and Redis-backed implementation for per-IP rate limiting.
package ratelimiter

import "time"

type Result struct {
	Allowed   bool
	Remaining int
	ResetAt   time.Time
}

// RateLimiter enforces per-key rate limits.
//
// Allow uses the limiter's configured default RPM (suitable for IP-based limiting).
// AllowN uses the caller-supplied rpm (suitable for per-client key-based limiting).
// Close releases any background resources held by the limiter.
type RateLimiter interface {
	Allow(key string) (Result, error)
	AllowN(key string, rpm int) (Result, error)
	Close() error
}
