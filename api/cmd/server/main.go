package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"zaas/api/internal/config"
	"zaas/api/internal/email"
	"zaas/api/internal/handler"
	"zaas/api/internal/ratelimiter"
	"zaas/api/internal/store"
	"zaas/api/internal/telemetry"

	"github.com/google/uuid"
)

var errRedisURLRequired = errors.New("ZAAS_RATE_LIMIT_BACKEND=redis requires ZAAS_REDIS_URL to be set")

// version is injected at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	cfg := config.Load()

	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	var h slog.Handler
	if cfg.LogFormat == "json" {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}
	logger := slog.New(h)
	slog.SetDefault(logger)

	if err := run(cfg, logger); err != nil {
		logger.ErrorContext(context.Background(), "fatal", "error", err)
		os.Exit(1)
	}
}

// initStore connects to PostgreSQL, runs migrations, and creates the store.
// Returns nil stores (and no error) if DatabaseURL is not configured.
func initStore(ctx context.Context, cfg config.Config, logger *slog.Logger) (*store.PostgresStore, store.ClientStore, func(), error) {
	if cfg.DatabaseURL == "" {
		logger.InfoContext(ctx, "no database URL configured - auth endpoints disabled")
		return nil, nil, func() {}, nil
	}
	pool, err := store.ConnectAndMigrate(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("database setup: %w", err)
	}
	logger.InfoContext(ctx, "database connected and migrations applied")
	pgStore := store.NewPostgresStore(pool)
	return pgStore, pgStore, pool.Close, nil
}

// initEmail creates an SMTP sender if host and store are configured.
func initEmail(cfg config.Config, pgStore *store.PostgresStore) email.Sender {
	if cfg.SMTPHost == "" || pgStore == nil {
		return nil
	}
	return email.NewSMTPSender(email.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		User:     cfg.SMTPUser,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	})
}

// initRateLimiter creates the appropriate rate limiter based on configuration.
// The caller must call the returned close function when done.
func initRateLimiter(ctx context.Context, cfg config.Config, logger *slog.Logger) (ratelimiter.RateLimiter, func(), error) {
	switch cfg.RateLimitBackend {
	case "redis":
		if cfg.RedisURL == "" {
			return nil, nil, errRedisURLRequired
		}
		redisRL, err := ratelimiter.NewRedisRateLimiter(cfg.RedisURL, cfg.RateLimitRPM)
		if err != nil {
			return nil, nil, fmt.Errorf("init redis rate limiter: %w", err)
		}
		logger.InfoContext(ctx, "redis rate limiter initialized", "url", cfg.RedisURL, "rpm", cfg.RateLimitRPM)
		return redisRL, func() { _ = redisRL.Close() }, nil
	default:
		rl := ratelimiter.NewMemoryRateLimiter(cfg.RateLimitRPM)
		logger.InfoContext(ctx, "memory rate limiter initialized", "rpm", cfg.RateLimitRPM)
		return rl, func() {
			if err := rl.Close(); err != nil {
				logger.ErrorContext(ctx, "memory rate limiter close error", "error", err)
			} else {
				logger.InfoContext(ctx, "memory rate limiter stopped")
			}
		}, nil
	}
}

func run(cfg config.Config, logger *slog.Logger) error {
	ctx := context.Background()
	otelResult, err := telemetry.Init(ctx, telemetry.Params{
		Enabled:     cfg.OTelEnabled,
		ServiceName: "zaas-api",
		Version:     version,
		Environment: cfg.Environment,
		InstanceID:  uuid.New().String(),
	})
	if err != nil {
		return fmt.Errorf("init telemetry: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := otelResult.Shutdown(shutdownCtx); err != nil {
			logger.ErrorContext(shutdownCtx, "telemetry shutdown error", "error", err)
		}
	}()

	if cfg.OTelEnabled {
		// Replace the default slog handler with an OTel bridge so that all
		// subsequent slog.Default() calls are exported to the OTel log pipeline
		// alongside traces and metrics.
		otelHandler := otelslog.NewHandler("zaas-api")
		bridgedLogger := slog.New(otelHandler)
		slog.SetDefault(bridgedLogger)
		logger = bridgedLogger
	}

	// Connect to PostgreSQL and set up store + email.
	pgStore, clientStore, closeDB, err := initStore(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer closeDB()

	emailSender := initEmail(cfg, pgStore)

	var deps handler.Deps
	if pgStore != nil {
		deps = handler.Deps{Store: pgStore, Tokens: pgStore, Email: emailSender}
	}
	srv := handler.New(cfg, deps)

	rl, closeRL, err := initRateLimiter(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer closeRL()
	router := handler.NewRouter(cfg, rl, otelResult.PrometheusHandler, srv, clientStore)

	return serve(ctx, cfg, logger, router)
}

// serve starts the HTTP server and blocks until a shutdown signal is received.
func serve(ctx context.Context, cfg config.Config, logger *slog.Logger, h http.Handler) error {
	httpSrv := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "zaas-api starting", "port", cfg.Port, "base_url", cfg.BaseURL)
		serverErr <- httpSrv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		logger.InfoContext(ctx, "shutting down", "signal", sig.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.InfoContext(shutdownCtx, "server stopped cleanly")
	return nil
}
