package telemetry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"azule.info/calorize/internal/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestInitTracer_UnsetEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	tp, err := telemetry.InitTracer(context.Background(), "calorize-api", "test-sha")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if tp != nil {
		t.Fatalf("expected nil TracerProvider when endpoint unset, got %v", tp)
	}
}

func TestOtelHTTPIntegration(t *testing.T) {
	origTP := otel.GetTracerProvider()
	origProp := otel.GetTextMapPropagator()

	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(origTP)
		otel.SetTextMapPropagator(origProp)
	})

	var capturedSpanCtx trace.SpanContext
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSpanCtx = trace.SpanContextFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := otelhttp.NewHandler(innerHandler, "calorize-api")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !capturedSpanCtx.IsValid() {
		t.Fatal("expected downstream handler context to have a valid span context")
	}
	if !capturedSpanCtx.HasTraceID() || capturedSpanCtx.TraceID().String() == "00000000000000000000000000000000" {
		t.Fatalf("expected non-empty trace ID, got %s", capturedSpanCtx.TraceID().String())
	}
}

func TestOtelHTTPPropagation(t *testing.T) {
	origTP := otel.GetTracerProvider()
	origProp := otel.GetTextMapPropagator()

	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(origTP)
		otel.SetTextMapPropagator(origProp)
	})

	var capturedSpanCtx trace.SpanContext
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSpanCtx = trace.SpanContextFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := otelhttp.NewHandler(innerHandler, "calorize-api")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	const incomingTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	req.Header.Set("traceparent", "00-"+incomingTraceID+"-00f067aa0ba902b7-01")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !capturedSpanCtx.IsValid() {
		t.Fatal("expected downstream handler context to have a valid span context")
	}
	if capturedSpanCtx.TraceID().String() != incomingTraceID {
		t.Fatalf("expected propagated trace ID %s, got %s", incomingTraceID, capturedSpanCtx.TraceID().String())
	}
}
