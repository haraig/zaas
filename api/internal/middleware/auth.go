// Package middleware provides HTTP middleware for the ZaaS API: authentication,
// CORS, rate limiting, request ID injection, and structured access logging.
package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"zaas/api/internal/store"
)

// Auth returns middleware that extracts and validates API keys.
// Anonymous requests (no Authorization header) pass through.
// Invalid/revoked keys receive HTTP 401.
func Auth(clients store.ClientStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// Anonymous request - set OTel attribute
				span := trace.SpanFromContext(r.Context())
				span.SetAttributes(attribute.String("zaas.auth", "anonymous"))
				next.ServeHTTP(w, r)
				return
			}

			// Accept "Bearer" scheme in any case (e.g. "bearer", "BEARER").
			if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				writeProblem(r, w, http.StatusUnauthorized, "INVALID_API_KEY",
					"Invalid or revoked API key. Visit https://zaas.at to register or re-issue a key.")
				return
			}
			key := authHeader[len("bearer "):]

			// If clients store is nil (no DB configured), reject all keys
			if clients == nil {
				writeProblem(r, w, http.StatusUnauthorized, "INVALID_API_KEY",
					"Invalid or revoked API key. Visit https://zaas.at to register or re-issue a key.")
				return
			}

			hash := sha256sum(key)

			client, err := clients.GetClientByAPIKeyHash(r.Context(), hash)
			if err != nil || client == nil || client.RevokedAt != nil || client.VerifiedAt == nil {
				writeProblem(r, w, http.StatusUnauthorized, "INVALID_API_KEY",
					"Invalid or revoked API key. Visit https://zaas.at to register or re-issue a key.")
				return
			}

			ctx := WithClient(r.Context(), ClientInfoFromStore(client))

			// OTel enrichment
			span := trace.SpanFromContext(ctx)
			span.SetAttributes(
				attribute.String("zaas.client_id", client.ID),
				attribute.String("zaas.client_name", client.DisplayName),
				attribute.String("zaas.auth", "key"),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// sha256sum returns the hex-encoded SHA-256 hash of s.
func sha256sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// writeProblem emits an RFC 9457 application/problem+json response from middleware.
func writeProblem(r *http.Request, w http.ResponseWriter, status int, code, detail string) {
	slugs := map[string]string{
		"RATE_LIMITED":        "rate-limited",
		"INVALID_PARAM":       "invalid-param",
		"INTERNAL_ERROR":      "internal-error",
		"INVALID_API_KEY":     "invalid-api-key",
		"SERVICE_UNAVAILABLE": "service-unavailable",
	}
	titles := map[string]string{
		"RATE_LIMITED":        "Too Many Requests",
		"INVALID_PARAM":       "Bad Request",
		"INTERNAL_ERROR":      "Internal Server Error",
		"INVALID_API_KEY":     "Unauthorized",
		"SERVICE_UNAVAILABLE": "Service Unavailable",
	}
	slug := slugs[code]
	if slug == "" {
		slug = "unknown"
	}
	title := titles[code]
	if title == "" {
		title = "Error"
	}
	instance := r.URL.Path
	reqID := chimiddleware.GetReqID(r.Context())

	body := map[string]any{
		"type":       fmt.Sprintf("https://zaas.at/errors/%s", slug),
		"title":      title,
		"status":     status,
		"detail":     detail,
		"instance":   instance,
		"code":       code,
		"request_id": reqID,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.ErrorContext(r.Context(), "writeProblem encode error", "error", err)
	}
}
