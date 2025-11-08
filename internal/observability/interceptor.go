package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	requestDuration metric.Float64Histogram
	requestCount    metric.Int64Counter
)

func init() {
	meter := otel.Meter("products-service")
	var err error

	requestDuration, err = meter.Float64Histogram(
		"grpc_request_duration_seconds",
		metric.WithDescription("Duration of gRPC requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(err)
	}

	requestCount, err = meter.Int64Counter(
		"grpc_requests_total",
		metric.WithDescription("Total number of gRPC requests"),
	)
	if err != nil {
		panic(err)
	}
}

func UnaryServerInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		// Start span
		ctx, span := otel.Tracer("products-service").Start(ctx, info.FullMethod)
		defer span.End()

		span.SetAttributes(
			attribute.String("grpc.method", info.FullMethod),
		)

		// Log request
		logger.Info("gRPC request started",
			zap.String("method", info.FullMethod),
			zap.Any("request", req),
		)

		// Handle request
		resp, err := handler(ctx, req)

		duration := time.Since(start).Seconds()

		// Record metrics
		statusCode := codes.OK
		if err != nil {
			if s, ok := status.FromError(err); ok {
				statusCode = s.Code()
			} else {
				statusCode = codes.Unknown
			}
		}

		requestDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("method", info.FullMethod),
				attribute.String("status", statusCode.String()),
			),
		)

		requestCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("method", info.FullMethod),
				attribute.String("status", statusCode.String()),
			),
		)

		// Update span
		span.SetAttributes(
			attribute.String("grpc.status", statusCode.String()),
			attribute.Float64("duration", duration),
		)

		if err != nil {
			span.RecordError(err)
			logger.Error("gRPC request failed",
				zap.String("method", info.FullMethod),
				zap.Error(err),
				zap.Duration("duration", time.Since(start)),
			)
		} else {
			logger.Info("gRPC request completed",
				zap.String("method", info.FullMethod),
				zap.Duration("duration", time.Since(start)),
			)
		}

		return resp, err
	}
}
