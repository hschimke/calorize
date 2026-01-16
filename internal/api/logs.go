package api

import (
	"encoding/json"
	"net/http"
	"time"

	"azule.info/calorize/internal/database"
)

func GetLogs(w http.ResponseWriter, r *http.Request) {
	// Query params: date (optional, default today)
	dateStr := r.URL.Query().Get("date")
	var err error
	var startTime, endTime time.Time

	if dateStr == "" {
		// Default to today (local or UTC? API should probably define. Defaults to server local or UTC)
		now := time.Now()
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		endTime = startTime.Add(24 * time.Hour)
	} else {
		// Parse YYYY-MM-DD
		startTime, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			http.Error(w, "invalid date format (YYYY-MM-DD)", http.StatusBadRequest)
			return
		}
		endTime = startTime.Add(24 * time.Hour)
	}

	ctx := r.Context()
	// Need userID. For now assuming global or single user if no auth middleware active in this MVP setup.
	// But we have Auth system.
	// Ideally we extract UserID from context (set by middleware).
	// For MVP, we'll look for "user_id" query param or header?
	// Or just "test-user-id" constant if not logged in.
	// Let's check cookie "auth_session_id" -> get user -> proceed.
	// Since I haven't implemented Middleware to populate Context, I'll do a quick lookup here or accept a header for testing.
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		// Fallback for testing: query param
		userID = r.URL.Query().Get("user_id")
	}
	if userID == "" {
		http.Error(w, "user_id required (header or query)", http.StatusUnauthorized)
		return
	}

	logs, err := database.GetLogsRange(ctx, userID, startTime, endTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(logs)
}

type CreateLogRequest struct {
	UserID  string  `json:"user_id"` // Optional if inferred from auth
	FoodID  string  `json:"food_id"`
	Amount  float64 `json:"amount"`
	MealTag string  `json:"meal_tag"`
}

func CreateLog(w http.ResponseWriter, r *http.Request) {
	var req CreateLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// Auth check fallback
	if req.UserID == "" {
		req.UserID = r.Header.Get("X-User-ID")
	}
	if req.UserID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		return
	}

	logEntry := &database.Log{
		UserID:   req.UserID,
		FoodID:   req.FoodID,
		Amount:   req.Amount,
		MealTag:  req.MealTag,
		LoggedAt: time.Now(), // Or user provided?
	}

	ctx := r.Context()
	err := database.CreateLog(ctx, logEntry)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(logEntry)
}

func DeleteLog(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Verify user owns log... skipped for MVP
	ctx := r.Context()
	if err := database.DeleteLog(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
