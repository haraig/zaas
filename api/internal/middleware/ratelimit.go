package middleware

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"zaas/api/internal/ratelimiter"
)

// rateLimitMetrics holds the OTel instruments for rate-limit observability.
type rateLimitMetrics struct {
	errors    metric.Int64Counter
	decisions metric.Int64Counter
}

func newRateLimitMetrics() rateLimitMetrics {
	meter := otel.GetMeterProvider().Meter("zaas/ratelimit")
	errors, _ := meter.Int64Counter("zaas_ratelimit_errors_total",
		metric.WithDescription("Number of rate-limiter backend errors"))
	decisions, _ := meter.Int64Counter("zaas_ratelimit_decision_total",
		metric.WithDescription("Number of rate-limit decisions made"))
	return rateLimitMetrics{errors: errors, decisions: decisions}
}

// RateLimit returns middleware that enforces rate limiting.
// For authenticated requests (client in context): uses AllowN with client's RPM.
// For anonymous requests: uses Allow with the configured default RPM per IP.
// When failClosed is true, a backend error returns 503 instead of passing the request through.
func RateLimit(rl ratelimiter.RateLimiter, rpm int, failClosed bool) func(http.Handler) http.Handler {
	m := newRateLimitMetrics()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var result ratelimiter.Result
			var err error
			var limit int
			var keyType string

			if client := GetClient(r.Context()); client != nil {
				// Authenticated: per-client rate limit
				result, err = rl.AllowN("client:"+client.ID, client.RateLimitRPM)
				limit = client.RateLimitRPM
				keyType = "client"
			} else {
				// Anonymous: per-IP rate limit
				ip := extractIP(r)
				result, err = rl.Allow(ip)
				limit = rpm
				keyType = "ip"
			}

			if err != nil {
				slog.ErrorContext(r.Context(), "rate limiter error",
					"error", err, "path", r.URL.Path, "key_type", keyType)
				m.errors.Add(r.Context(), 1, metric.WithAttributes(
					attribute.String("key_type", keyType),
				))
				if failClosed {
					w.Header().Set("Retry-After", "5")
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(http.StatusServiceUnavailable)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"type":        "https://zaas.at/errors/service-unavailable",
						"title":       "Service Unavailable",
						"status":      503,
						"detail":      "Rate limiter temporarily unavailable. Please retry shortly.",
						"instance":    r.URL.Path,
						"code":        "SERVICE_UNAVAILABLE",
						"retry_after": 5,
						"request_id":  chimiddleware.GetReqID(r.Context()),
					})
					return
				}
				// Non-auth routes: fail open to preserve availability.
				next.ServeHTTP(w, r)
				return
			}

			allowed := result.Allowed
			m.decisions.Add(r.Context(), 1, metric.WithAttributes(
				attribute.String("key_type", keyType),
				attribute.Bool("allowed", allowed),
			))

			resetUnix := result.ResetAt.Unix()
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetUnix))

			if !result.Allowed {
				retryAfter := int(time.Until(result.ResetAt).Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"type":        "https://zaas.at/errors/rate-limited",
					"title":       "Too Many Requests",
					"status":      429,
					"detail":      fmt.Sprintf("Rate limit exceeded (%d req/min). Email contact@zaas.at to request a free API key for higher limits.", limit),
					"instance":    r.URL.Path,
					"code":        "RATE_LIMITED",
					"retry_after": retryAfter,
					"request_id":  chimiddleware.GetReqID(r.Context()),
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractIP returns the client IP for rate-limit keying.
//
// Source-of-truth strategy: trust the rightmost entry of X-Forwarded-For,
// which is the hop appended by the trusted Caddy reverse proxy. A spoofed
// first entry supplied by the client is therefore ignored. When X-Forwarded-For
// is absent (e.g., direct local connections) the host portion of RemoteAddr is
// used, with net.SplitHostPort so that IPv6 addresses like [::1]:1234 are
// handled correctly (brackets are stripped by SplitHostPort).
//
// See docs/reference/architecture.md for the rationale behind this choice.
func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Use the rightmost entry: the hop added by Caddy (trusted proxy).
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[len(parts)-1])
		if ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr has no port (unusual) - use as-is.
		return r.RemoteAddr
	}
	return host
}
