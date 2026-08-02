// Package telemetry initializes the OpenTelemetry SDK for tracing, metrics,
// and logging with an OTLP gRPC exporter.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	otellogglobal "go.opentelemetry.io/otel/log/global"
	lognoop "go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/metric/noop"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// Params holds the configuration for OTel SDK initialization.
type Params struct {
	// Enabled controls whether real providers are created.
	// When false, all providers are no-ops.
	Enabled bool

	// ServiceName is the logical name of the service (e.g. "zaas-api").
	ServiceName string

	// Version is the deployed version string, typically injected via -ldflags.
	Version string

	// Environment is the deployment environment (e.g. "production", "staging").
	Environment string

	// InstanceID uniquely identifies this service instance.
	InstanceID string
}

// Result holds the shutdown function and optional Prometheus scrape handler.
// PrometheusHandler is non-nil only when OTel is enabled.
// Mount it at /metrics to expose Prometheus-format metrics.
// Resource is non-nil only when OTel is enabled.
type Result struct {
	Shutdown          func(context.Context) error
	PrometheusHandler http.Handler
	Resource          *sdkresource.Resource
}

// Init initializes the OpenTelemetry SDK for traces, metrics, and logs.
// When Params.Enabled is false, all providers are replaced with no-op implementations.
// The sampler ratio is read from OTEL_TRACES_SAMPLER_ARG (float64, 0.0-1.0; default 1.0).
func Init(ctx context.Context, p Params) (Result, error) {
	if !p.Enabled {
		otel.SetTracerProvider(tracenoop.NewTracerProvider())
		otel.SetMeterProvider(noop.NewMeterProvider())
		otellogglobal.SetLoggerProvider(lognoop.NewLoggerProvider())
		return Result{Shutdown: func(context.Context) error { return nil }}, nil
	}

	res, err := buildResource(ctx, p)
	if err != nil {
		return Result{}, fmt.Errorf("build OTel resource: %w", err)
	}

	traceExp, err := otlptracegrpc.New(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("create trace exporter: %w", err)
	}
	metricExp, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("create metric exporter: %w", err)
	}
	logExp, err := otlploggrpc.New(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("create log exporter: %w", err)
	}
	promReg := prometheus.NewRegistry()
	// A fresh registry does not include Go runtime / process metrics the way
	// prometheus.DefaultRegisterer does - register them explicitly so the
	// Grafana dashboard's Goroutines/Memory/GC panels have data to query.
	promReg.MustRegister(collectors.NewGoCollector())
	promReg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	promExp, err := promexporter.New(promexporter.WithRegisterer(promReg))
	if err != nil {
		return Result{}, fmt.Errorf("create prometheus exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(buildSampler()),
	)
	otel.SetTracerProvider(tp)

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithReader(promExp),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)
	otellogglobal.SetLoggerProvider(lp)

	return Result{
		Shutdown:          shutdownProviders(tp, mp, lp),
		PrometheusHandler: promhttp.HandlerFor(promReg, promhttp.HandlerOpts{}),
		Resource:          res,
	}, nil
}

// shutdownProviders returns a shutdown function that gracefully stops all three providers.
func shutdownProviders(
	tp *sdktrace.TracerProvider,
	mp *sdkmetric.MeterProvider,
	lp *sdklog.LoggerProvider,
) func(context.Context) error {
	return func(ctx context.Context) error {
		var errs []error
		if err := tp.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		if err := mp.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		if err := lp.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			return fmt.Errorf("telemetry shutdown: %w", errors.Join(errs...))
		}
		return nil
	}
}

// buildResource creates the OTel resource with standard service attributes.
func buildResource(ctx context.Context, p Params) (*sdkresource.Resource, error) {
	var kvs []attribute.KeyValue
	if p.ServiceName != "" {
		kvs = append(kvs, semconv.ServiceName(p.ServiceName))
	}
	if p.Version != "" {
		kvs = append(kvs, semconv.ServiceVersion(p.Version))
	}
	if p.Environment != "" {
		kvs = append(kvs, semconv.DeploymentEnvironment(p.Environment))
	}
	if p.InstanceID != "" {
		kvs = append(kvs, semconv.ServiceInstanceID(p.InstanceID))
	}

	// Note: combining WithSchemaURL and WithHost() causes a schema URL merge conflict.
	// We set attributes explicitly instead. See docs/reference/gotchas.md.
	return sdkresource.New(ctx, sdkresource.WithAttributes(kvs...))
}

// buildSampler returns a ParentBased(TraceIDRatioBased(ratio)) sampler.
// The ratio is read from OTEL_TRACES_SAMPLER_ARG; defaults to 1.0 (always sample).
func buildSampler() sdktrace.Sampler {
	ratio := 1.0
	if v := os.Getenv("OTEL_TRACES_SAMPLER_ARG"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			ratio = f
		}
	}
	return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
}
