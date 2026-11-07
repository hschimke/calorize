package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"azule.info/calorize/internal/db"
	"github.com/google/uuid"
)

func RegisterWeightPaths(mux *http.ServeMux) {
	mux.HandleFunc("GET /weight", getWeightLogsHandler)
	mux.HandleFunc("POST /weight", createWeightLogHandler)
	mux.HandleFunc("PUT /weight/{id}", updateWeightLogHandler)
	mux.HandleFunc("DELETE /weight/{id}", deleteWeightLogHandler)
	mux.HandleFunc("GET /weight/stats", getWeightStatsHandler)
}

type weightLogRequest struct {
	Weight   float64   `json:"weight"`
	Unit     string    `json:"unit"`
	LoggedAt time.Time `json:"logged_at"`
}

func getWeightLogsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	limit := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = val
		}
	}

	logs, err := db.GetWeightLogs(userID, limit)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func createWeightLogHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		WriteError(w, r, db.ErrNotFound)
		return
	}

	var req weightLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Weight <= 0 {
		http.Error(w, "weight must be a positive number", http.StatusBadRequest)
		return
	}

	if req.Unit == "" {
		if user.WeightUnit != "" {
			req.Unit = user.WeightUnit
		} else {
			req.Unit = "kg"
		}
	} else if req.Unit != "kg" && req.Unit != "lbs" {
		http.Error(w, "unit must be 'kg' or 'lbs'", http.StatusBadRequest)
		return
	}

	if req.LoggedAt.IsZero() {
		req.LoggedAt = time.Now().UTC()
	} else {
		req.LoggedAt = req.LoggedAt.UTC()
	}

	log, err := db.CreateWeightLog(db.WeightLog{
		UserID:   userID,
		Weight:   req.Weight,
		Unit:     req.Unit,
		LoggedAt: req.LoggedAt,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(log)
}

func updateWeightLogHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	logIDStr := r.PathValue("id")
	logID, err := uuid.Parse(logIDStr)
	if err != nil {
		http.Error(w, "Invalid log ID", http.StatusBadRequest)
		return
	}

	// Check existence & ownership
	existing, err := db.GetWeightLog(db.WeightLogID(logID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			WriteError(w, r, db.ErrNotFound)
			return
		}
		WriteError(w, r, err)
		return
	}
	if existing == nil {
		WriteError(w, r, db.ErrNotFound)
		return
	}
	if existing.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req weightLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Weight <= 0 {
		http.Error(w, "weight must be a positive number", http.StatusBadRequest)
		return
	}

	if req.Unit != "" && req.Unit != "kg" && req.Unit != "lbs" {
		http.Error(w, "unit must be 'kg' or 'lbs'", http.StatusBadRequest)
		return
	}

	if req.Unit == "" {
		req.Unit = existing.Unit
	}

	if req.LoggedAt.IsZero() {
		req.LoggedAt = existing.LoggedAt
	} else {
		req.LoggedAt = req.LoggedAt.UTC()
	}

	log, err := db.UpdateWeightLog(db.WeightLogID(logID), db.WeightLog{
		UserID:   userID,
		Weight:   req.Weight,
		Unit:     req.Unit,
		LoggedAt: req.LoggedAt,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(log)
}

func deleteWeightLogHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	logIDStr := r.PathValue("id")
	logID, err := uuid.Parse(logIDStr)
	if err != nil {
		http.Error(w, "Invalid log ID", http.StatusBadRequest)
		return
	}

	existing, err := db.GetWeightLog(db.WeightLogID(logID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			WriteError(w, r, db.ErrNotFound)
			return
		}
		WriteError(w, r, err)
		return
	}
	if existing == nil {
		WriteError(w, r, db.ErrNotFound)
		return
	}
	if existing.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := db.DeleteWeightLog(db.WeightLogID(logID), userID); err != nil {
		WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type weightStatsResponse struct {
	CurrentWeight float64  `json:"current_weight"`
	StartWeight   float64  `json:"start_weight"`
	WeightChange  float64  `json:"weight_change"`
	GoalWeight    *float64 `json:"goal_weight"`
	WeightUnit    string   `json:"weight_unit"`
	GoalProgress  float64  `json:"goal_progress"`
}

func getWeightStatsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		WriteError(w, r, db.ErrNotFound)
		return
	}

	logs, err := db.GetWeightLogs(userID, 0)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	prefUnit := user.WeightUnit
	if prefUnit == "" {
		prefUnit = "kg"
	}

	var resp weightStatsResponse
	resp.WeightUnit = prefUnit
	if user.WeightGoal != nil {
		resp.GoalWeight = user.WeightGoal
	}

	if len(logs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Logs are ordered by logged_at DESC, created_at DESC.
	// So logs[0] is the most recent (CurrentWeight).
	// logs[len(logs)-1] is the earliest (StartWeight).
	currentLog := logs[0]
	startLog := logs[len(logs)-1]

	currentW := convertWeight(currentLog.Weight, currentLog.Unit, prefUnit)
	startW := convertWeight(startLog.Weight, startLog.Unit, prefUnit)

	resp.CurrentWeight = currentW
	resp.StartWeight = startW
	resp.WeightChange = currentW - startW

	if user.WeightGoal != nil {
		goalW := *user.WeightGoal
		if startW == goalW {
			resp.GoalProgress = 100.0
		} else if startW > goalW {
			// Lose weight goal
			if currentW <= goalW {
				resp.GoalProgress = 100.0
			} else if currentW >= startW {
				resp.GoalProgress = 0.0
			} else {
				resp.GoalProgress = ((startW - currentW) / (startW - goalW)) * 100.0
			}
		} else {
			// Gain weight goal
			if currentW >= goalW {
				resp.GoalProgress = 100.0
			} else if currentW <= startW {
				resp.GoalProgress = 0.0
			} else {
				resp.GoalProgress = ((currentW - startW) / (goalW - startW)) * 100.0
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func convertWeight(value float64, fromUnit, toUnit string) float64 {
	if fromUnit == toUnit || fromUnit == "" || toUnit == "" {
		return value
	}
	if fromUnit == "kg" && toUnit == "lbs" {
		return value * 2.20462
	}
	if fromUnit == "lbs" && toUnit == "kg" {
		return value / 2.20462
	}
	return value
}
