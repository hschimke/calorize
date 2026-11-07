package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"azule.info/calorize/internal/api"
)

func TestProbes(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterProbePaths(mux)

	t.Run("GET /healthz returns 200 ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", contentType)
		}
		expectedBody := `{"status":"ok"}`
		if rec.Body.String() != expectedBody {
			t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
		}
	})

	t.Run("GET /readyz returns 200 ready when db healthy", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", contentType)
		}
		expectedBody := `{"status":"ready"}`
		if rec.Body.String() != expectedBody {
			t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
		}
	})

	t.Run("GET /readyz returns 503 unavailable when ping fails", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // canceled context causes PingContext to fail

		req := httptest.NewRequest(http.MethodGet, "/readyz", nil).WithContext(ctx)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status 503, got %d", rec.Code)
		}
		if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", contentType)
		}
		expectedBody := `{"status":"unavailable","error":"database unreachable"}`
		if rec.Body.String() != expectedBody {
			t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
		}
	})
}
