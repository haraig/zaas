package config_test

import (
	"os"
	"testing"

	"zaas/api/internal/config"
)

func TestDefaults(t *testing.T) {
	t.Parallel()
	for _, k := range []string{
		"ZAAS_PORT", "ZAAS_BASE_URL", "ZAAS_LOG_LEVEL", "ZAAS_LOG_FORMAT",
		"ZAAS_CORS_ORIGINS", "ZAAS_RATE_LIMIT_RPM",
		"ZAAS_RATE_LIMIT_BACKEND", "ZAAS_REDIS_URL",
		"ZAAS_OTEL_ENABLED", "ZAAS_METRICS_ENDPOINT_ENABLED",
		"ZAAS_SMTP_HOST", "ZAAS_SMTP_PORT", "ZAAS_SMTP_USER", "ZAAS_SMTP_PASSWORD", "ZAAS_SMTP_FROM",
		"ZAAS_ADMIN_TOKEN",
	} {
		os.Unsetenv(k)
	}

	cfg := config.Load()

	if cfg.Port != "8080" {
		t.Errorf("Port: got %q, want %q", cfg.Port, "8080")
	}
	if cfg.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL: got %q, want %q", cfg.BaseURL, "http://localhost:8080")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat: got %q, want %q", cfg.LogFormat, "text")
	}
	if cfg.CORSOrigins != "*" {
		t.Errorf("CORSOrigins: got %q, want %q", cfg.CORSOrigins, "*")
	}
	if cfg.RateLimitRPM != 60 {
		t.Errorf("RateLimitRPM: got %d, want 60", cfg.RateLimitRPM)
	}
	if cfg.RateLimitBackend != "memory" {
		t.Errorf("RateLimitBackend: got %q, want %q", cfg.RateLimitBackend, "memory")
	}
	if cfg.OTelEnabled != true {
		t.Errorf("OTelEnabled: got %v, want true", cfg.OTelEnabled)
	}
	if cfg.MetricsEndpointEnabled != false {
		t.Errorf("MetricsEndpointEnabled: got %v, want false", cfg.MetricsEndpointEnabled)
	}
	if cfg.SMTPHost != "smtp.migadu.com" {
		t.Errorf("SMTPHost: got %q, want %q", cfg.SMTPHost, "smtp.migadu.com")
	}
	if cfg.SMTPPort != 587 {
		t.Errorf("SMTPPort: got %d, want 587", cfg.SMTPPort)
	}
	if cfg.SMTPUser != "" {
		t.Errorf("SMTPUser: got %q, want %q", cfg.SMTPUser, "")
	}
	if cfg.SMTPPassword != "" {
		t.Errorf("SMTPPassword: got %q, want %q", cfg.SMTPPassword, "")
	}
	if cfg.SMTPFrom != "ZaaS <noreply@zaas.at>" {
		t.Errorf("SMTPFrom: got %q, want %q", cfg.SMTPFrom, "ZaaS <noreply@zaas.at>")
	}
	if cfg.AdminToken != "" {
		t.Errorf("AdminToken: got %q, want %q", cfg.AdminToken, "")
	}
}

func TestOverrides(t *testing.T) {
	t.Parallel()
	os.Setenv("ZAAS_PORT", "9090")
	os.Setenv("ZAAS_BASE_URL", "https://zaas.at")
	os.Setenv("ZAAS_RATE_LIMIT_RPM", "120")
	os.Setenv("ZAAS_OTEL_ENABLED", "false")
	os.Setenv("ZAAS_METRICS_ENDPOINT_ENABLED", "true")
	os.Setenv("ZAAS_ADMIN_TOKEN", "test-admin-token")
	defer func() {
		os.Unsetenv("ZAAS_PORT")
		os.Unsetenv("ZAAS_BASE_URL")
		os.Unsetenv("ZAAS_RATE_LIMIT_RPM")
		os.Unsetenv("ZAAS_OTEL_ENABLED")
		os.Unsetenv("ZAAS_METRICS_ENDPOINT_ENABLED")
		os.Unsetenv("ZAAS_ADMIN_TOKEN")
	}()

	cfg := config.Load()

	if cfg.Port != "9090" {
		t.Errorf("Port: got %q, want %q", cfg.Port, "9090")
	}
	if cfg.BaseURL != "https://zaas.at" {
		t.Errorf("BaseURL: got %q, want %q", cfg.BaseURL, "https://zaas.at")
	}
	if cfg.RateLimitRPM != 120 {
		t.Errorf("RateLimitRPM: got %d, want 120", cfg.RateLimitRPM)
	}
	if cfg.OTelEnabled != false {
		t.Errorf("OTelEnabled: got %v, want false", cfg.OTelEnabled)
	}
	if cfg.MetricsEndpointEnabled != true {
		t.Errorf("MetricsEndpointEnabled: got %v, want true", cfg.MetricsEndpointEnabled)
	}
	if cfg.AdminToken != "test-admin-token" {
		t.Errorf("AdminToken: got %q, want %q", cfg.AdminToken, "test-admin-token")
	}
}
