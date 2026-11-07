package telemetry

import (
	"context"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTracer initializes OpenTelemetry tracing with an OTLP gRPC exporter.
// If OTEL_EXPORTER_OTLP_ENDPOINT is unset or empty, tracing is disabled and (nil, nil) is returned.
func InitTracer(ctx context.Context, serviceName, gitSHA string) (*sdktrace.TracerProvider, error) {
	rawEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if rawEndpoint == "" {
		// Tracing disabled if no collector endpoint provided
		return nil, nil
	}

	endpoint := rawEndpoint
	isInsecure := true
	if strings.HasPrefix(endpoint, "https://") {
		isInsecure = false
		endpoint = strings.TrimPrefix(endpoint, "https://")
	} else if strings.HasPrefix(endpoint, "http://") {
		endpoint = strings.TrimPrefix(endpoint, "http://")
	}

	if envInsecure := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"); envInsecure != "" {
		isInsecure = envInsecure == "true" || envInsecure == "1"
	}

	var grpcOpts []otlptracegrpc.Option
	grpcOpts = append(grpcOpts, otlptracegrpc.WithEndpoint(endpoint))
	if isInsecure {
		grpcOpts = append(grpcOpts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, grpcOpts...)
	if err != nil {
		return nil, err
	}

	customRes, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(gitSHA),
		),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(resource.Default(), customRes)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}
