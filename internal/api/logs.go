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
	userID, ok := GetUserID(ctx)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
	UserID   string    `json:"user_id"` // Optional if inferred from auth
	FoodID   string    `json:"food_id"`
	Amount   float64   `json:"amount"`
	MealTag  string    `json:"meal_tag"`
	LoggedAt time.Time `json:"logged_at"` // Optional
}

func CreateLog(w http.ResponseWriter, r *http.Request) {
	var req CreateLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// Auth check fallback
	// Auth check
	userID, ok := GetUserID(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	req.UserID = userID // Enforce authenticated user

	// Use provided time or default to now
	loggedAt := req.LoggedAt
	if loggedAt.IsZero() {
		loggedAt = time.Now()
	}

	logEntry := &database.Log{
		UserID:   req.UserID,
		FoodID:   req.FoodID,
		Amount:   req.Amount,
		MealTag:  req.MealTag,
		LoggedAt: loggedAt,
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
