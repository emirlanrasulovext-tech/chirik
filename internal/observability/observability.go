package observability

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/chirik/products/internal/config"
	clientprom "github.com/prometheus/client_golang/prometheus"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.uber.org/zap"
)

var (
	tracerProvider *trace.TracerProvider
	meterProvider  *metric.MeterProvider
)

func Init(cfg *config.Config, logger *zap.Logger) (func(), error) {
	ctx := context.Background()

	// Initialize resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("products-service"),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize tracer
	tp, err := initTracer(cfg, res, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tracer: %w", err)
	}
	tracerProvider = tp
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Initialize metrics
	mp, err := initMetrics(cfg, res, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}
	meterProvider = mp
	otel.SetMeterProvider(mp)

	// Start metrics server
	go startMetricsServer(cfg.MetricsPort, logger)

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if tracerProvider != nil {
			if err := tracerProvider.Shutdown(ctx); err != nil {
				logger.Error("Error shutting down tracer provider", zap.Error(err))
			}
		}

		if meterProvider != nil {
			if err := meterProvider.Shutdown(ctx); err != nil {
				logger.Error("Error shutting down meter provider", zap.Error(err))
			}
		}
	}

	return shutdown, nil
}

func initTracer(cfg *config.Config, res *resource.Resource, logger *zap.Logger) (*trace.TracerProvider, error) {
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.JaegerEndpoint)))
	if err != nil {
		return nil, fmt.Errorf("failed to create jaeger exporter: %w", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)

	logger.Info("Tracer initialized", zap.String("endpoint", cfg.JaegerEndpoint))
	return tp, nil
}

var prometheusExporter *otelprometheus.Exporter

func initMetrics(cfg *config.Config, res *resource.Resource, logger *zap.Logger) (*metric.MeterProvider, error) {
	exporter, err := otelprometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	prometheusExporter = exporter

	mp := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(res),
	)

	logger.Info("Metrics initialized", zap.String("port", cfg.MetricsPort))
	return mp, nil
}

func startMetricsServer(port string, logger *zap.Logger) {
	if prometheusExporter == nil {
		logger.Error("Prometheus exporter not initialized")
		return
	}

	// The OpenTelemetry prometheus exporter implements clientprom.Gatherer interface
	// We need to use type assertion to access it
	var gatherer clientprom.Gatherer
	if g, ok := interface{}(prometheusExporter).(clientprom.Gatherer); ok {
		gatherer = g
	} else {
		// Fallback: use default registry if exporter doesn't implement Gatherer
		gatherer = clientprom.DefaultGatherer
		logger.Warn("Prometheus exporter doesn't implement Gatherer, using default registry")
	}

	http.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))
	addr := fmt.Sprintf(":%s", port)
	logger.Info("Starting metrics server", zap.String("address", addr))
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Error("Metrics server failed", zap.Error(err))
	}
}
