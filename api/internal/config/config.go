// Package config loads ZaaS service configuration from environment variables.
package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                   string
	BaseURL                string
	LogLevel               string
	LogFormat              string
	CORSOrigins            string
	RateLimitRPM           int
	RateLimitBackend       string
	RedisURL               string
	DatabaseURL            string
	OTelEnabled            bool
	Environment            string
	MetricsEndpointEnabled bool
	SMTPHost               string
	SMTPPort               int
	SMTPUser               string
	SMTPPassword           string
	SMTPFrom               string
	AdminToken             string
}

func Load() Config {
	return Config{
		Port:                   getEnv("ZAAS_PORT", "8080"),
		BaseURL:                getEnv("ZAAS_BASE_URL", "http://localhost:8080"),
		LogLevel:               getEnv("ZAAS_LOG_LEVEL", "info"),
		LogFormat:              getEnv("ZAAS_LOG_FORMAT", "text"),
		CORSOrigins:            getEnv("ZAAS_CORS_ORIGINS", "*"),
		RateLimitRPM:           getEnvInt("ZAAS_RATE_LIMIT_RPM", 60),
		RateLimitBackend:       getEnv("ZAAS_RATE_LIMIT_BACKEND", "memory"),
		RedisURL:               getEnv("ZAAS_REDIS_URL", ""),
		DatabaseURL:            getEnv("ZAAS_DB_URL", ""),
		OTelEnabled:            getEnvBool("ZAAS_OTEL_ENABLED", true),
		Environment:            getEnv("ZAAS_ENVIRONMENT", "development"),
		MetricsEndpointEnabled: getEnvBool("ZAAS_METRICS_ENDPOINT_ENABLED", false),
		SMTPHost:               getEnv("ZAAS_SMTP_HOST", "smtp.migadu.com"),
		SMTPPort:               getEnvInt("ZAAS_SMTP_PORT", 587),
		SMTPUser:               getEnv("ZAAS_SMTP_USER", ""),
		SMTPPassword:           getEnv("ZAAS_SMTP_PASSWORD", ""),
		SMTPFrom:               getEnv("ZAAS_SMTP_FROM", "ZaaS <noreply@zaas.at>"),
		AdminToken:             getEnv("ZAAS_ADMIN_TOKEN", ""),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func getEnvBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}
