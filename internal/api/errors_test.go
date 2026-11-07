package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"azule.info/calorize/internal/api"
	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/db"
	"azule.info/calorize/internal/middleware"
	"github.com/google/uuid"
)

func TestWriteErrorMapping(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name:        "not found",
			err:         db.ErrNotFound,
			wantStatus:  http.StatusNotFound,
			wantCode:    "NOT_FOUND",
			wantMessage: "record not found",
		},
		{
			name:        "credential not found",
			err:         db.ErrCredentialNotFound,
			wantStatus:  http.StatusNotFound,
			wantCode:    "NOT_FOUND",
			wantMessage: "credential not found",
		},
		{
			name:        "wrapped not found",
			err:         fmt.Errorf("query failed: %w", db.ErrNotFound),
			wantStatus:  http.StatusNotFound,
			wantCode:    "NOT_FOUND",
			wantMessage: "query failed: record not found",
		},
		{
			name:        "conflict",
			err:         db.ErrConflict,
			wantStatus:  http.StatusConflict,
			wantCode:    "CONFLICT",
			wantMessage: "resource conflict",
		},
		{
			name:        "invalid input",
			err:         db.ErrInvalidInput,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "BAD_REQUEST",
			wantMessage: "invalid input",
		},
		{
			name:        "unexpected error hides internal message",
			err:         errors.New("db disk locked"),
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "INTERNAL_ERROR",
			wantMessage: "An unexpected error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rr := httptest.NewRecorder()
			api.WriteError(rr, req, tt.err)

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}
			if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("expected Content-Type application/json, got %q", ct)
			}

			var resp api.ErrorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response JSON: %v, body: %s", err, rr.Body.String())
			}

			if resp.Code != tt.wantCode {
				t.Fatalf("expected code %q, got %q", tt.wantCode, resp.Code)
			}
			if resp.Error != tt.wantMessage {
				t.Fatalf("expected error message %q, got %q", tt.wantMessage, resp.Error)
			}
		})
	}
}

func TestWriteErrorIncludesRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := context.WithValue(req.Context(), middleware.RequestIDKey, "test-req-id-1234")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	api.WriteError(rr, req, errors.New("boom"))

	var resp api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if resp.RequestID != "test-req-id-1234" {
		t.Fatalf("expected request_id %q, got %q", "test-req-id-1234", resp.RequestID)
	}
}

func TestWriteErrorOmitsEmptyRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	api.WriteError(rr, req, db.ErrNotFound)

	var raw map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if _, ok := raw["request_id"]; ok {
		t.Fatalf("expected request_id to be omitted when empty, got %v", raw["request_id"])
	}
}

func TestGetFoodNotFoundReturnsJSONError(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterApiPaths(mux)

	req := httptest.NewRequest(http.MethodGet, "/foods/00000000-0000-0000-0000-000000000001", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var resp api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v, body: %s", err, rr.Body.String())
	}
	if resp.Code != "NOT_FOUND" {
		t.Fatalf("expected code NOT_FOUND, got %q", resp.Code)
	}
}

func TestUserProfileNotFoundReturnsJSONError(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterApiPaths(mux)

	req := httptest.NewRequest(http.MethodGet, "/account/profile", nil)
	ctx := context.WithValue(req.Context(), auth.UserIDContextKey, db.UserID(uuid.New()))
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var resp api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v, body: %s", err, rr.Body.String())
	}
	if resp.Code != "NOT_FOUND" {
		t.Fatalf("expected code NOT_FOUND, got %q", resp.Code)
	}
}

func TestWeightLogNotFoundReturnsJSONError(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterApiPaths(mux)

	req := httptest.NewRequest(http.MethodDelete, "/weight/00000000-0000-0000-0000-000000000001", nil)
	ctx := context.WithValue(req.Context(), auth.UserIDContextKey, db.UserID(uuid.New()))
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var resp api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v, body: %s", err, rr.Body.String())
	}
	if resp.Code != "NOT_FOUND" {
		t.Fatalf("expected code NOT_FOUND, got %q", resp.Code)
	}
}


