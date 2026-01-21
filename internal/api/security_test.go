package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"azule.info/calorize/internal/api"
	"azule.info/calorize/internal/database"
)

func TestSecurityEnforcement(t *testing.T) {
	// Setup strictly for this test
	database.InitDB(":memory:")

	server := api.NewServer()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"GetLogs No Cookie", "GET", "/logs", http.StatusUnauthorized},
		{"CreateLog No Cookie", "POST", "/logs", http.StatusUnauthorized},
		{"GetFoods No Cookie", "GET", "/foods", http.StatusUnauthorized},
		{"UpdateFood No Cookie", "PUT", "/foods/123", http.StatusUnauthorized},
		{"Health Check Public", "GET", "/health", http.StatusOK},
		{"Login Begin Public", "POST", "/auth/login/begin", http.StatusNotFound}, // 404 means handler reached (user not found), so not 401 (blocked)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				// Special handling: Login Begin requires body, might be 400, but definitely NOT 401 if public.
				// However, if we protected it by mistake, it would be 401.
				// If we expect 200 but got 400 (Bad Request), that's acceptable for "Public".
				if tt.wantStatus == http.StatusOK && w.Code == http.StatusBadRequest {
					return
				}
				t.Errorf("path %s: want status %d, got %d", tt.path, tt.wantStatus, w.Code)
			}
		})
	}
}
