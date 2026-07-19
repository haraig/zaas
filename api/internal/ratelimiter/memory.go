package ratelimiter

import (
	"sync"
	"time"
)

// cleanupInterval and entryTTL bound memory: entries idle for 10 minutes are
// evicted every 5 minutes.
const cleanupInterval = 5 * time.Minute
const entryTTL = 10 * time.Minute

// windowDuration is the sliding window size for rate limiting.
const windowDuration = time.Minute

type windowEntry struct {
	timestamps []time.Time
	lastSeen   time.Time
}

// MemoryRateLimiter is an in-process per-key sliding window rate limiter.
// It stores request timestamps per key and counts those within the last minute.
// Call Close to stop the background cleanup goroutine.
type MemoryRateLimiter struct {
	mu        sync.Mutex
	entries   map[string]*windowEntry
	rpm       int
	stop      chan struct{}
	closeOnce sync.Once
}

func NewMemoryRateLimiter(rpm int) *MemoryRateLimiter {
	rl := &MemoryRateLimiter{
		entries: make(map[string]*windowEntry),
		rpm:     rpm,
		stop:    make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Allow uses the limiter's configured default RPM.
func (rl *MemoryRateLimiter) Allow(key string) (Result, error) {
	return rl.AllowN(key, rl.rpm)
}

// AllowN uses the caller-supplied rpm, enabling per-client rate limits.
// If rpm <= 0, all requests are denied immediately.
func (rl *MemoryRateLimiter) AllowN(key string, rpm int) (Result, error) {
	if rpm <= 0 {
		return Result{Allowed: false, Remaining: 0, ResetAt: time.Now().Add(windowDuration)}, nil
	}

	now := time.Now()
	windowStart := now.Add(-windowDuration)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	e := rl.getOrCreate(key)
	e.lastSeen = now

	// Evict timestamps outside the window.
	valid := e.timestamps[:0]
	for _, ts := range e.timestamps {
		if ts.After(windowStart) {
			valid = append(valid, ts)
		}
	}
	e.timestamps = valid

	count := len(e.timestamps)
	allowed := count < rpm

	if allowed {
		e.timestamps = append(e.timestamps, now)
		count++
	}

	remaining := rpm - count
	if remaining < 0 {
		remaining = 0
	}

	// ResetAt: when the oldest timestamp in the window exits (window slides past it).
	var resetAt time.Time
	if len(e.timestamps) > 0 {
		resetAt = e.timestamps[0].Add(windowDuration)
	} else {
		resetAt = now.Add(windowDuration)
	}

	return Result{
		Allowed:   allowed,
		Remaining: remaining,
		ResetAt:   resetAt,
	}, nil
}

func (rl *MemoryRateLimiter) getOrCreate(key string) *windowEntry {
	e, ok := rl.entries[key]
	if !ok {
		e = &windowEntry{}
		rl.entries[key] = e
	}
	return e
}

func (rl *MemoryRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stop:
			return
		}
	}
}

// Close stops the background cleanup goroutine. It is safe to call multiple times.
func (rl *MemoryRateLimiter) Close() error {
	rl.closeOnce.Do(func() { close(rl.stop) })
	return nil
}

func (rl *MemoryRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-entryTTL)
	for key, e := range rl.entries {
		if e.lastSeen.Before(cutoff) {
			delete(rl.entries, key)
		}
	}
}
