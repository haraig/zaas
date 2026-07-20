package ratelimiter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// slidingWindowLua is an atomic Lua script for Redis sliding window rate limiting.
//
// KEYS[1]: the sorted set key (e.g. "rate:ip:1.2.3.4")
// ARGV[1]: current Unix timestamp in milliseconds
// ARGV[2]: window size in milliseconds (60000 for 1-minute window)
// ARGV[3]: rate limit (max requests per window)
// ARGV[4]: unique member ID for this request (timestamp_ms:random)
//
// Returns: {allowed (0|1), count_after, oldest_ts_ms_or_0}.
const slidingWindowLua = `
local key     = KEYS[1]
local now     = tonumber(ARGV[1])
local window  = tonumber(ARGV[2])
local limit   = tonumber(ARGV[3])
local member  = ARGV[4]

-- Remove timestamps outside the window
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)

-- Count requests currently in the window
local count = redis.call('ZCARD', key)

local allowed = 0
if count < limit then
    -- Add this request
    redis.call('ZADD', key, now, member)
    redis.call('PEXPIRE', key, window)
    allowed = 1
    count = count + 1
end

-- Oldest timestamp in window (for ResetAt calculation)
local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
local oldest_ts = 0
if #oldest > 0 then
    oldest_ts = tonumber(oldest[2])
end

return {allowed, count, oldest_ts}
`

// ErrUnexpectedResult is returned when the Redis Lua script returns an unexpected number of values
// or values of unexpected types.
var ErrUnexpectedResult = errors.New("redis sliding window: unexpected result length")

// ErrInvalidResultType is returned when the Redis Lua script returns a value that cannot be
// asserted to the expected int64 type.
var ErrInvalidResultType = errors.New("redis sliding window: unexpected result type")

// RedisRateLimiter is a distributed per-key sliding window rate limiter backed by Redis.
type RedisRateLimiter struct {
	client *redis.Client
	rpm    int
	script *redis.Script
}

// NewRedisRateLimiter creates a RedisRateLimiter connected to the given Redis URL.
// rpm is the default rate limit used by Allow; AllowN accepts a caller-supplied rpm.
func NewRedisRateLimiter(redisURL string, rpm int) (*RedisRateLimiter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	client := redis.NewClient(opts)

	if err := redisotel.InstrumentTracing(client); err != nil {
		return nil, fmt.Errorf("instrument redis tracing: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &RedisRateLimiter{
		client: client,
		rpm:    rpm,
		script: redis.NewScript(slidingWindowLua),
	}, nil
}

// Allow uses the limiter's configured default RPM.
func (rl *RedisRateLimiter) Allow(key string) (Result, error) {
	return rl.AllowN(key, rl.rpm)
}

// AllowN uses the caller-supplied rpm, enabling per-client dynamic rate limits.
// If rpm <= 0, all requests are denied immediately.
func (rl *RedisRateLimiter) AllowN(key string, rpm int) (Result, error) {
	if rpm <= 0 {
		return Result{Allowed: false, Remaining: 0, ResetAt: time.Now().Add(time.Minute)}, nil
	}

	now := time.Now()
	nowMs := now.UnixMilli()
	windowMs := int64(time.Minute / time.Millisecond)
	redisKey := fmt.Sprintf("rate:%s", key)
	member := fmt.Sprintf("%d:%s", nowMs, randomHex(8))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := rl.script.Run(ctx, rl.client,
		[]string{redisKey},
		nowMs, windowMs, rpm, member,
	).Slice()
	if err != nil {
		return Result{}, fmt.Errorf("redis sliding window: %w", err)
	}

	allowed, count, oldestMs, err := parseScriptResult(res)
	if err != nil {
		return Result{}, err
	}

	remaining := rpm - count
	if remaining < 0 {
		remaining = 0
	}

	var resetAt time.Time
	if oldestMs > 0 {
		resetAt = time.UnixMilli(oldestMs).Add(time.Minute)
	} else {
		resetAt = now.Add(time.Minute)
	}

	return Result{
		Allowed:   allowed,
		Remaining: remaining,
		ResetAt:   resetAt,
	}, nil
}

// Close releases the Redis client connection.
func (rl *RedisRateLimiter) Close() error {
	return rl.client.Close()
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// parseScriptResult decodes the three-element slice returned by slidingWindowLua.
// Returns (allowed, count, oldestMs, error). Returns ErrUnexpectedResult if the
// slice length is wrong and ErrInvalidResultType if any element cannot be asserted
// to int64 - both conditions are bugs in the Lua script or unexpected Redis behavior.
func parseScriptResult(res []interface{}) (allowed bool, count int, oldestMs int64, err error) {
	if len(res) != 3 {
		return false, 0, 0, ErrUnexpectedResult
	}
	allowedRaw, ok := res[0].(int64)
	if !ok {
		return false, 0, 0, ErrInvalidResultType
	}
	countRaw, ok := res[1].(int64)
	if !ok {
		return false, 0, 0, ErrInvalidResultType
	}
	oldestMsRaw, ok := res[2].(int64)
	if !ok {
		return false, 0, 0, ErrInvalidResultType
	}
	return allowedRaw == 1, int(countRaw), oldestMsRaw, nil
}
