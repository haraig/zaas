package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"zaas/api/internal/config"
	"zaas/api/internal/gen"
	"zaas/api/internal/middleware"
	"zaas/api/internal/ratelimiter"
	"zaas/api/internal/store"
)

// NewRouter wires the chi router with all middleware and handlers.
// promHandler: the Prometheus scrape handler from telemetry.Init (nil = disabled).
// The entire router is wrapped with otelhttp for automatic trace span creation.
func NewRouter(cfg config.Config, rl ratelimiter.RateLimiter, promHandler http.Handler, srv *Server, clientStore store.ClientStore) http.Handler {
	r := chi.NewRouter()

	logger := slog.Default()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.ClientIPFromHeader("X-Real-Ip"))
	r.Use(middleware.Logger(logger))
	r.Use(middleware.CORS(cfg.CORSOrigins))

	r.Get("/healthz", Healthz)
	r.Get("/readyz", Readyz)

	// Prometheus metrics scrape endpoint (enabled via ZAAS_METRICS_ENDPOINT_ENABLED).
	if cfg.MetricsEndpointEnabled {
		r.Get("/metrics", MetricsHandler(promHandler))
	}

	// Auth endpoints - aggressive per-IP rate limiting, no API key required.
	r.Group(func(r chi.Router) {
		authRL := ratelimiter.NewMemoryRateLimiter(5)
		r.Use(middleware.RateLimit(authRL, 5, cfg.BaseURL, true))
		authWrapper := &gen.ServerInterfaceWrapper{
			Handler: srv,
			ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", err.Error(), 0)
			},
		}
		r.Post("/api/v1/auth/register", srv.AuthRegister)
		r.Post("/api/v1/auth/verify", authWrapper.AuthVerify)
		r.Post("/api/v1/auth/reissue", srv.AuthReissue)
	})

	// Main API - auth middleware + standard rate limiting.
	// We register only the randomness endpoints here (not auth routes) to avoid conflict.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(clientStore))
		r.Use(middleware.RateLimit(rl, cfg.RateLimitRPM, cfg.BaseURL, false))
		r.Route("/api/v1", func(r chi.Router) {
			wrapper := &gen.ServerInterfaceWrapper{
				Handler: srv,
				ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
					WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", err.Error(), 0)
				},
			}
			r.Get("/coin", wrapper.FlipCoin)
			r.Get("/color", wrapper.GenerateColor)
			r.Get("/coordinates", wrapper.GenerateCoordinates)
			r.Get("/dice", wrapper.RollDice)
			r.Get("/lorem", wrapper.GenerateLorem)
			r.Get("/number", wrapper.GenerateNumber)
			r.Get("/openapi.yaml", wrapper.GetOpenAPISpec)
			r.Get("/password", wrapper.GeneratePassword)
			r.Get("/ssh-key", wrapper.GenerateSSHKey)
			r.Get("/uuid", wrapper.GenerateUUID)
			r.Get("/words", wrapper.GenerateWords)
		})
	})

	return otelhttp.NewHandler(r, "zaas-api",
		otelhttp.WithFilter(func(req *http.Request) bool {
			// Skip telemetry for health-check probes - they generate noise.
			p := req.URL.Path
			return p != "/healthz" && p != "/readyz"
		}),
		otelhttp.WithSpanNameFormatter(func(_ string, req *http.Request) string {
			// Use METHOD /path without query string so ?token= values never
			// appear in trace span names or OTel HTTP attributes.
			return req.Method + " " + req.URL.Path
		}),
	)
}
