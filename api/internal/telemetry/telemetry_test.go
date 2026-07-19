package telemetry_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	otellogglobal "go.opentelemetry.io/otel/log/global"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"zaas/api/internal/telemetry"
)

func TestInit_NoOpWhenDisabled(t *testing.T) {
	t.Parallel()
	result, err := telemetry.Init(context.Background(), telemetry.Params{Enabled: false})
	if err != nil {
		t.Fatalf("Init(disabled) returned error: %v", err)
	}
	if result.Shutdown == nil {
		t.Error("shutdown function should not be nil")
	}
	if err := result.Shutdown(context.Background()); err != nil {
		t.Errorf("shutdown() returned unexpected error: %v", err)
	}
	if otel.GetTracerProvider() == nil {
		t.Error("TracerProvider should not be nil")
	}
	if otel.GetMeterProvider() == nil {
		t.Error("MeterProvider should not be nil")
	}
	if otellogglobal.GetLoggerProvider() == nil {
		t.Error("LoggerProvider should not be nil")
	}
}

func TestInit_PrometheusHandlerNilWhenDisabled(t *testing.T) {
	t.Parallel()
	result, err := telemetry.Init(context.Background(), telemetry.Params{Enabled: false})
	if err != nil {
		t.Fatalf("Init(disabled) error: %v", err)
	}
	if result.PrometheusHandler != nil {
		t.Error("PrometheusHandler should be nil when OTel disabled")
	}
}

func TestInit_PrometheusHandlerNotNilWhenEnabled(t *testing.T) {
	t.Parallel()
	// The exporter connects lazily - no running collector needed.
	result, err := telemetry.Init(context.Background(), telemetry.Params{
		Enabled:     true,
		ServiceName: "test-svc",
		Version:     "1.2.3",
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("Init(enabled) error: %v", err)
	}
	defer result.Shutdown(context.Background()) //nolint:errcheck
	if result.PrometheusHandler == nil {
		t.Error("PrometheusHandler should not be nil when OTel enabled")
	}
}

func TestInit_ResourceAttributesIncludeVersionAndEnvironment(t *testing.T) {
	t.Parallel()
	result, err := telemetry.Init(context.Background(), telemetry.Params{
		Enabled:     true,
		ServiceName: "zaas-test",
		Version:     "2.0.0",
		Environment: "staging",
		InstanceID:  "test-instance-1",
	})
	if err != nil {
		t.Fatalf("Init(enabled) error: %v", err)
	}
	defer result.Shutdown(context.Background()) //nolint:errcheck

	if result.Resource == nil {
		t.Fatal("Resource should not be nil when OTel enabled")
	}

	attrs := result.Resource.Attributes()
	found := map[string]bool{}
	for _, a := range attrs {
		switch a.Key {
		case semconv.ServiceVersionKey:
			if a.Value.AsString() != "2.0.0" {
				t.Errorf("service.version = %q; want %q", a.Value.AsString(), "2.0.0")
			}
			found["version"] = true
		case semconv.DeploymentEnvironmentKey:
			if a.Value.AsString() != "staging" {
				t.Errorf("deployment.environment = %q; want %q", a.Value.AsString(), "staging")
			}
			found["env"] = true
		case semconv.ServiceInstanceIDKey:
			if a.Value.AsString() != "test-instance-1" {
				t.Errorf("service.instance.id = %q; want %q", a.Value.AsString(), "test-instance-1")
			}
			found["instance"] = true
		case semconv.ServiceNameKey:
			if a.Value.AsString() != "zaas-test" {
				t.Errorf("service.name = %q; want %q", a.Value.AsString(), "zaas-test")
			}
			found["name"] = true
		}
	}

	for _, key := range []string{"version", "env", "instance", "name"} {
		if !found[key] {
			t.Errorf("resource attribute %q not found in %v", key, attrs)
		}
	}
}

func TestInit_ResourceNilWhenDisabled(t *testing.T) {
	t.Parallel()
	result, err := telemetry.Init(context.Background(), telemetry.Params{Enabled: false})
	if err != nil {
		t.Fatalf("Init(disabled) error: %v", err)
	}
	if result.Resource != nil {
		t.Error("Resource should be nil when OTel disabled")
	}
}
