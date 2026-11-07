package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"azule.info/calorize/internal/api"
	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/telemetry"
	"github.com/google/uuid"
	dto "github.com/prometheus/client_model/go"
)

func getDroppedMetricValue(t *testing.T, reason string) float64 {
	m := &dto.Metric{}
	err := telemetry.ClientEventsDroppedTotal.WithLabelValues(reason).Write(m)
	if err != nil {
		t.Fatalf("failed to read metric: %v", err)
	}
	return m.GetCounter().GetValue()
}

func TestClientEventsSanitizesURLs(t *testing.T) {
	api.ResetClientEventsLimiter()
	mux := http.NewServeMux()
	api.RegisterClientEventsPaths(mux)

	payload := `{"events":[{"kind":"js_error","message":"Error at https://example.com/app.js?token=secret#top in line 5","location":"https://example.com/bundle.js?auth=xyz#section","route":"/dashboard.html?debug=true#tab1"}]}`
	req := httptest.NewRequest("POST", "/client-events", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify SanitizeString helper
	sanitized := api.SanitizeString("https://example.com/app.js?token=secret#top")
	if sanitized != "https://example.com/app.js" {
		t.Errorf("expected sanitized URL without params/hash, got %q", sanitized)
	}
}

func TestClientEventsTruncation(t *testing.T) {
	api.ResetClientEventsLimiter()
	mux := http.NewServeMux()
	api.RegisterClientEventsPaths(mux)

	longString := strings.Repeat("a", 600)
	sanitized := api.SanitizeString(longString)
	if len(sanitized) != 500 {
		t.Fatalf("expected truncated length 500, got %d", len(sanitized))
	}

	payload := fmt.Sprintf(`{"events":[{"kind":"js_error","message":"%s","location":"loc","route":"/home"}]}`, longString)
	req := httptest.NewRequest("POST", "/client-events", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestClientEventsBodySizeLimit(t *testing.T) {
	api.ResetClientEventsLimiter()
	mux := http.NewServeMux()
	api.RegisterClientEventsPaths(mux)

	// Construct body > 16 KiB (16384 bytes)
	hugeString := strings.Repeat("x", 17000)
	payload := fmt.Sprintf(`{"events":[{"kind":"js_error","message":"%s","location":"loc","route":"/home"}]}`, hugeString)

	req := httptest.NewRequest("POST", "/client-events", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 400 or 413 for oversized body (>16KiB), got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestClientEventsBatchSizeLimit(t *testing.T) {
	api.ResetClientEventsLimiter()
	mux := http.NewServeMux()
	api.RegisterClientEventsPaths(mux)

	// Batch of 21 events (> 20 limit)
	events := make([]map[string]string, 21)
	for i := range events {
		events[i] = map[string]string{
			"kind":    "js_error",
			"message": fmt.Sprintf("Error %d", i),
			"route":   "/dashboard.html",
		}
	}
	body, _ := json.Marshal(map[string]any{"events": events})

	req := httptest.NewRequest("POST", "/client-events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for batch > 20 events, got %d: %s", rr.Code, rr.Body.String())
	}

	// Batch of 20 events (exact limit, should be accepted)
	events20 := make([]map[string]string, 20)
	for i := range events20 {
		events20[i] = map[string]string{
			"kind":    "js_error",
			"message": fmt.Sprintf("Error %d", i),
			"route":   "/dashboard.html",
		}
	}
	body20, _ := json.Marshal(map[string]any{"events": events20})

	req20 := httptest.NewRequest("POST", "/client-events", bytes.NewReader(body20))
	req20.Header.Set("Content-Type", "application/json")
	rr20 := httptest.NewRecorder()

	mux.ServeHTTP(rr20, req20)
	if rr20.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted for batch of 20 events, got %d: %s", rr20.Code, rr20.Body.String())
	}
}

func TestClientEventsRateLimitingAnonymous(t *testing.T) {
	api.ResetClientEventsLimiter()
	mux := http.NewServeMux()
	api.RegisterClientEventsPaths(mux)

	beforeDrops := getDroppedMetricValue(t, "rate_limit")

	payload := `{"events":[{"kind":"js_error","message":"test","route":"/index"}]}`
	clientIP := "198.51.100.1"

	// 10 requests should succeed for this IP (limit is 10/min)
	for i := 1; i <= 10; i++ {
		req := httptest.NewRequest("POST", "/client-events", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", clientIP)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusAccepted {
			t.Fatalf("request %d expected 202 Accepted, got %d: %s", i, rr.Code, rr.Body.String())
		}
	}

	// 11th request should be rate-limited (HTTP 429)
	req := httptest.NewRequest("POST", "/client-events", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", clientIP)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("11th request expected 429 Too Many Requests, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["code"] != "RATE_LIMITED" {
		t.Errorf("expected code RATE_LIMITED, got %q", resp["code"])
	}

	afterDrops := getDroppedMetricValue(t, "rate_limit")
	if afterDrops <= beforeDrops {
		t.Errorf("expected ClientEventsDroppedTotal to increment, before: %f, after: %f", beforeDrops, afterDrops)
	}

	// A different IP should still be allowed
	reqOther := httptest.NewRequest("POST", "/client-events", bytes.NewBufferString(payload))
	reqOther.Header.Set("Content-Type", "application/json")
	reqOther.Header.Set("X-Forwarded-For", "198.51.100.2")
	rrOther := httptest.NewRecorder()
	mux.ServeHTTP(rrOther, reqOther)
	if rrOther.Code != http.StatusAccepted {
		t.Fatalf("request from different IP expected 202 Accepted, got %d: %s", rrOther.Code, rrOther.Body.String())
	}
}

func TestClientEventsRateLimitingAuthenticated(t *testing.T) {
	api.ResetClientEventsLimiter()
	mux := http.NewServeMux()
	api.RegisterClientEventsPaths(mux)

	userID := uuid.New()
	payload := `{"events":[{"kind":"js_error","message":"test","route":"/index"}]}`

	// 30 requests should succeed for this user (limit is 30/min)
	for i := 1; i <= 30; i++ {
		req := httptest.NewRequest("POST", "/client-events", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), auth.UserIDContextKey, userID)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusAccepted {
			t.Fatalf("user request %d expected 202 Accepted, got %d: %s", i, rr.Code, rr.Body.String())
		}
	}

	// 31st request should be rate-limited (HTTP 429)
	req := httptest.NewRequest("POST", "/client-events", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), auth.UserIDContextKey, userID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("31st user request expected 429 Too Many Requests, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["code"] != "RATE_LIMITED" {
		t.Errorf("expected code RATE_LIMITED, got %q", resp["code"])
	}
}

func TestClientEventsRoutingWithPrefix(t *testing.T) {
	api.ResetClientEventsLimiter()
	v1Mux := http.NewServeMux()
	api.RegisterClientEventsPaths(v1Mux)

	mainMux := http.NewServeMux()
	mainMux.Handle("/api/v1/", http.StripPrefix("/api/v1", v1Mux))

	payload := `{"events":[{"kind":"js_error","message":"prefix test","route":"/dashboard.html"}]}`
	req := httptest.NewRequest("POST", "/api/v1/client-events", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mainMux.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted at /api/v1/client-events, got %d: %s", rr.Code, rr.Body.String())
	}
}
