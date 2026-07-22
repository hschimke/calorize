package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/db"
	"github.com/google/uuid"
)

func getUserID(r *http.Request) (db.UserID, error) {
	v := r.Context().Value(auth.UserIDContextKey)
	if v == nil {
		return db.UserID(uuid.Nil), fmt.Errorf("no user id in context")
	}
	uid, ok := v.(db.UserID)
	if !ok {
		return db.UserID(uuid.Nil), fmt.Errorf("invalid user id type in context")
	}
	return uid, nil
}

func RegisterApiPaths(mux *http.ServeMux) {
	RegisterLogsPaths(mux)
	RegisterFoodsPaths(mux)
	RegisterStatsPaths(mux)
	RegisterAccountPaths(mux)
	RegisterWeightPaths(mux)
}

// ### Foods
// - GET /foods
//     - Returns list of current versions
// - POST /foods
//     - Create new food/recipe
//     - Payload: { name, calories, protein, carbs, fat, type, measurement_unit, measurement_amount, nutrients: [], ingredients: {} }
// - GET /foods/{id}
//     - Returns details including sub-ingredients if recipe
// - PUT /foods/{id}
//     - Updates a food by creating a NEW Version
//     - Payload: Same as POST
// - DELETE /foods/{id}
//     - Soft delete

// { name, calories, protein, carbs, fat, type, measurement_unit, measurement_amount, nutrients: [], ingredients: {} }
type createFoodRequest struct {
	Name              string             `json:"name"`
	Calories          float64            `json:"calories"`
	Protein           float64            `json:"protein"`
	Carbs             float64            `json:"carbs"`
	Fat               float64            `json:"fat"`
	Type              string             `json:"type"`
	MeasurementUnit   string             `json:"measurement_unit"`
	MeasurementAmount float64            `json:"measurement_amount"`
	Servings          float64            `json:"servings"`
	Nutrients         []db.FoodNutrient  `json:"nutrients"`
	Ingredients       map[string]float64 `json:"ingredients"`
}

func RegisterFoodsPaths(mux *http.ServeMux) {
	mux.HandleFunc("GET /foods", getFoodsHandler)
	mux.HandleFunc("POST /foods", createFoodHandler)
	mux.HandleFunc("GET /foods/{id}", getFoodHandler)
	mux.HandleFunc("PUT /foods/{id}", updateFoodHandler)
	mux.HandleFunc("DELETE /foods/{id}", deleteFoodHandler)
	mux.HandleFunc("POST /foods/{id}/copy", copyFoodHandler)
	mux.HandleFunc("GET /foods/{id}/lineage", getFoodLineageHandler)
}

// copyFoodHandler duplicates a visible food into the caller's own foods,
// recording copy lineage. Pure 1:1 copy — no request body; rename or adjust
// via the normal update flow afterward.
func copyFoodHandler(w http.ResponseWriter, r *http.Request) {
	foodIDString := r.PathValue("id")
	foodID, err := uuid.Parse(foodIDString)
	if err != nil {
		http.Error(w, "Invalid food ID", http.StatusBadRequest)
		return
	}

	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	source, err := db.GetFood(db.FoodID(foodID))
	if err != nil {
		http.Error(w, "Failed to get food", http.StatusInternalServerError)
		return
	}
	// 404 for missing, deleted, and invisible foods alike — don't leak existence.
	if source == nil || !(source.CreatorID == db.UserID(uuid.Nil) || source.Public || source.CreatorID == userID) {
		http.Error(w, "Food not found", http.StatusNotFound)
		return
	}

	food, err := db.CopyFood(db.FoodID(foodID), userID)
	if err != nil {
		slog.Error("failed to copy food", "error", err, "id", foodID)
		http.Error(w, "Failed to copy food", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(food); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

// getFoodLineageHandler returns the copy lineage of a food: the chain of
// foods it was copied from (nearest-first) and the full copy tree rooted at
// the lineage's origin. Foods the caller may not see appear as redacted stubs.
func getFoodLineageHandler(w http.ResponseWriter, r *http.Request) {
	foodIDString := r.PathValue("id")
	foodID, err := uuid.Parse(foodIDString)
	if err != nil {
		http.Error(w, "Invalid food ID", http.StatusBadRequest)
		return
	}

	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	food, err := db.GetFood(db.FoodID(foodID))
	if err != nil {
		http.Error(w, "Failed to get food", http.StatusInternalServerError)
		return
	}
	if food == nil {
		http.Error(w, "Food not found", http.StatusNotFound)
		return
	}

	lineage, err := db.GetFoodLineage(db.FoodID(foodID), userID)
	if err != nil {
		slog.Error("failed to get food lineage", "error", err, "id", foodID)
		http.Error(w, "Failed to get food lineage", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(lineage); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func getFoodsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}
	disabledSources, err := db.GetDisabledSources(userID)
	if err != nil {
		slog.Error("failed to get disabled sources", "error", err)
		http.Error(w, "Failed to get foods", http.StatusInternalServerError)
		return
	}

	var (
		foods []db.Food
		dbErr error
	)
	if r.URL.Query().Has("recent") {
		// Source filters intentionally not applied: recent foods are from the user's own
		// log history; hiding a food the user already logged would be disorienting.
		foods, dbErr = db.GetRecentFoods(userID, 50)
	} else if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		foods, dbErr = db.SearchFoods(userID, q, 20, disabledSources, user.HidePublicUserFoods)
	} else if r.URL.Query().Has("mine") {
		// Source filters intentionally not applied: "mine" returns only user-created foods,
		// which are never tagged with an import source.
		foods, dbErr = db.GetUserFoods(userID)
	} else {
		foods, dbErr = db.GetFoods(userID, disabledSources, user.HidePublicUserFoods)
	}
	if dbErr != nil {
		slog.Error("failed to get foods", "error", dbErr)
		http.Error(w, "Failed to get foods", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(foods); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func createFoodHandler(w http.ResponseWriter, r *http.Request) {
	var req createFoodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Food name is required", http.StatusBadRequest)
		return
	}

	var ingredients []db.RecipeItems
	for id, amount := range req.Ingredients {
		foodID, err := uuid.Parse(id)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid ingredient ID: %s", id), http.StatusBadRequest)
			return
		}
		ingredients = append(ingredients, db.RecipeItems{
			IngredientID: db.FoodID(foodID),
			Amount:       amount,
		})
	}

	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	food, err := db.CreateFood(db.Food{
		CreatorID:         userID,
		Name:              req.Name,
		Calories:          req.Calories,
		Protein:           req.Protein,
		Carbs:             req.Carbs,
		Fat:               req.Fat,
		Type:              req.Type,
		MeasurementUnit:   req.MeasurementUnit,
		MeasurementAmount: req.MeasurementAmount,
		Servings:          req.Servings,
		Ingredients:       ingredients,
		Nutrients:         req.Nutrients,
		Public:            true, // Default to public
	})
	if err != nil {
		http.Error(w, "Failed to create food", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(food); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func getFoodHandler(w http.ResponseWriter, r *http.Request) {
	foodIDString := r.PathValue("id")
	foodID, err := uuid.Parse(foodIDString)
	if err != nil {
		http.Error(w, "Invalid food ID", http.StatusBadRequest)
		return
	}
	food, err := db.GetFood(db.FoodID(foodID))
	if err != nil {
		http.Error(w, "Failed to get food", http.StatusInternalServerError)
		return
	}
	if food == nil {
		http.Error(w, "Food not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(food); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func updateFoodHandler(w http.ResponseWriter, r *http.Request) {
	foodIDString := r.PathValue("id")
	foodID, err := uuid.Parse(foodIDString)
	if err != nil {
		http.Error(w, "Invalid food ID", http.StatusBadRequest)
		return
	}

	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check ownership
	existing, err := db.GetFood(db.FoodID(foodID))
	if err != nil {
		http.Error(w, "Failed to check food ownership", http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "Food not found", http.StatusNotFound)
		return
	}
	if existing.CreatorID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req createFoodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Food name is required", http.StatusBadRequest)
		return
	}

	var ingredients []db.RecipeItems
	for id, amount := range req.Ingredients {
		foodID, err := uuid.Parse(id)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid ingredient ID: %s", id), http.StatusBadRequest)
			return
		}
		ingredients = append(ingredients, db.RecipeItems{
			IngredientID: db.FoodID(foodID),
			Amount:       amount,
		})
	}

	food, err := db.UpdateFood(db.FoodID(foodID), db.Food{
		CreatorID:         userID,
		Name:              req.Name,
		Calories:          req.Calories,
		Protein:           req.Protein,
		Carbs:             req.Carbs,
		Fat:               req.Fat,
		Type:              req.Type,
		MeasurementUnit:   req.MeasurementUnit,
		MeasurementAmount: req.MeasurementAmount,
		Servings:          req.Servings,
		Ingredients:       ingredients,
		Nutrients:         req.Nutrients,
		Public:            existing.Public, // Keep existing visibility
	})
	if err != nil {
		http.Error(w, "Failed to update food", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(food); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func deleteFoodHandler(w http.ResponseWriter, r *http.Request) {
	foodIDString := r.PathValue("id")
	foodID, err := uuid.Parse(foodIDString)
	if err != nil {
		http.Error(w, "Invalid food ID", http.StatusBadRequest)
		return
	}

	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check ownership
	existing, err := db.GetFood(db.FoodID(foodID))
	if err != nil {
		http.Error(w, "Failed to check food", http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "Food not found", http.StatusNotFound)
		return
	}
	if existing.CreatorID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := db.DeleteFood(db.FoodID(foodID)); err != nil {
		slog.Error("failed to delete food", "error", err, "id", foodID)
		http.Error(w, "Failed to delete food", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func getClientLocation(r *http.Request) *time.Location {
	tzOffset := r.URL.Query().Get("tz_offset")
	if tzOffset == "" {
		return time.UTC
	}
	offsetMins, err := strconv.Atoi(tzOffset)
	if err != nil {
		slog.Warn("invalid tz_offset, defaulting to UTC", "tz_offset", tzOffset)
		return time.UTC
	}
	return time.FixedZone("Client", -offsetMins*60)
}

// ### Stats
// - GET /stats
//     - Query Params: ?period={day,week,month}&date=YYYY-MM-DD
//     - Returns aggregated macros and total calories

func RegisterStatsPaths(mux *http.ServeMux) {
	mux.HandleFunc("GET /stats", getStatsHandler)
	mux.HandleFunc("GET /stats/breakdown", getStatsBreakdownHandler)
	mux.HandleFunc("GET /stats/consistency", getConsistencyStatsHandler)
}

func getStatsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	dateStr := r.URL.Query().Get("date")
	loc := getClientLocation(r)

	date := time.Now().In(loc)
	if dateStr != "" {
		parsed, err := time.ParseInLocation("2006-01-02", dateStr, loc)
		if err != nil {
			http.Error(w, "Invalid date format, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		date = parsed
	}

	stats, err := db.GetStats(userID, r.URL.Query().Get("period"), date)
	if err != nil {
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func getStatsBreakdownHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "day" || period == "" {
		http.Error(w, "period must be 'week' or 'month'", http.StatusBadRequest)
		return
	}

	dateStr := r.URL.Query().Get("date")
	loc := getClientLocation(r)

	date := time.Now().In(loc)
	if dateStr != "" {
		parsed, err := time.ParseInLocation("2006-01-02", dateStr, loc)
		if err != nil {
			http.Error(w, "Invalid date format, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		date = parsed
	}

	tzOffset := 0
	if s := r.URL.Query().Get("tz_offset"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			tzOffset = v
		}
	}

	breakdown, err := db.GetStatsByDay(userID, period, date, tzOffset)
	if err != nil {
		slog.Error("failed to get stats breakdown", "error", err)
		http.Error(w, "Failed to get stats breakdown", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(breakdown); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func getConsistencyStatsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	tzOffset := 0
	if s := r.URL.Query().Get("tz_offset"); s != "" {
		if v, parseErr := strconv.Atoi(s); parseErr == nil {
			tzOffset = v
		}
	}

	calorieGoal := 0
	if user.CalorieGoal != nil {
		calorieGoal = *user.CalorieGoal
	}

	stats, err := db.GetConsistencyStats(userID, tzOffset, calorieGoal, time.Now())
	if err != nil {
		slog.Error("failed to get consistency stats", "error", err)
		http.Error(w, "Failed to get consistency stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

// ### Logs
// - GET /logs
//     - Query Params: ?date=YYYY-MM-DD (Defaults to today)
//     - Returns logs for the day
// - POST /logs
//     - Create log entry
//     - Payload: { food_id, amount, meal_tag, logged_at (optional) }
// - POST /logs/copy
//     - Copy log entries from one date to another
//     - Payload: { from_date, to_date, meal_tags }
// - DELETE /logs/{id}

func RegisterLogsPaths(mux *http.ServeMux) {
	mux.HandleFunc("GET /logs", getLogsHandler)
	mux.HandleFunc("POST /logs", createLogEntryHandler)
	mux.HandleFunc("POST /logs/copy", copyLogEntriesHandler)
	mux.HandleFunc("GET /logs/{id}/lineage", getLogLineageHandler)
	mux.HandleFunc("DELETE /logs/{id}", deleteLogEntryHandler)
}

// getLogLineageHandler returns a summary of a log entry's copy provenance:
// the origin entry its day-copy chain started from and the number of
// copy-steps in between. Only the caller's own entries are visible.
func getLogLineageHandler(w http.ResponseWriter, r *http.Request) {
	entryIDString := r.PathValue("id")
	entryID, err := uuid.Parse(entryIDString)
	if err != nil {
		http.Error(w, "Invalid log entry ID", http.StatusBadRequest)
		return
	}

	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	summary, err := db.GetFoodLogLineageSummary(db.FoodLogEntryID(entryID), userID)
	if err != nil {
		slog.Error("failed to get log entry lineage", "error", err, "id", entryID)
		http.Error(w, "Failed to get log entry lineage", http.StatusInternalServerError)
		return
	}
	if summary == nil {
		http.Error(w, "Log entry not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(summary); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func getLogsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	loc := getClientLocation(r)

	date := time.Now().In(loc)
	if d := r.URL.Query().Get("date"); d != "" {
		parsed, err := time.ParseInLocation("2006-01-02", d, loc)
		if err != nil {
			http.Error(w, "Invalid date format, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		date = parsed
	}

	logs, err := db.GetFoodLogEntries(userID, date)
	if err != nil {
		http.Error(w, "Failed to get logs", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(logs); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

type createLogEntryRequest struct {
	FoodID      *db.FoodID `json:"food_id"`
	PortionName *string    `json:"portion_name"`
	Calories    *float64   `json:"calories"`
	Note        *string    `json:"note"`
	Amount      float64    `json:"amount"`
	MealTag     string     `json:"meal_tag"`
	LoggedAt    time.Time  `json:"logged_at"`
}

func createLogEntryHandler(w http.ResponseWriter, r *http.Request) {
	var req createLogEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if req.FoodID == nil && req.Calories == nil {
		http.Error(w, "Either food_id or calories must be provided", http.StatusBadRequest)
		return
	}

	// Normalize note: trim whitespace, treat empty as nil; notes only apply to quick entries
	if req.Note != nil {
		trimmed := strings.TrimSpace(*req.Note)
		if trimmed == "" {
			req.Note = nil
		} else {
			req.Note = &trimmed
		}
	}
	if req.FoodID != nil {
		req.Note = nil
	}
	if req.Note != nil && len([]rune(*req.Note)) > 100 {
		http.Error(w, "Note must be 100 characters or fewer", http.StatusBadRequest)
		return
	}

	if req.LoggedAt.IsZero() {
		req.LoggedAt = time.Now().UTC()
	} else {
		req.LoggedAt = req.LoggedAt.UTC()
	}

	entry, err := db.CreateFoodLogEntry(db.FoodLogEntry{
		UserID:      userID,
		FoodID:      req.FoodID,
		PortionName: req.PortionName,
		Calories:    req.Calories,
		Note:        req.Note,
		Amount:      req.Amount,
		MealTag:     req.MealTag,
		LoggedAt:    req.LoggedAt,
	})
	if err != nil {
		http.Error(w, "Failed to create log entry", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entry); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func deleteLogEntryHandler(w http.ResponseWriter, r *http.Request) {
	logEntryIdString := r.PathValue("id")
	logEntryId, err := uuid.Parse(logEntryIdString)
	if err != nil {
		http.Error(w, "Invalid log entry ID", http.StatusBadRequest)
		return
	}
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err := db.DeleteFoodLogEntry(db.FoodLogEntryID(logEntryId), userID); err != nil {
		slog.Error("failed to delete log entry", "error", err, "id", logEntryId)
		http.Error(w, "Failed to delete log entry", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type copyLogEntriesRequest struct {
	FromDate string   `json:"from_date"`
	ToDate   string   `json:"to_date"`
	MealTags []string `json:"meal_tags"`
}

func copyLogEntriesHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req copyLogEntriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.MealTags) == 0 {
		http.Error(w, "meal_tags must not be empty", http.StatusBadRequest)
		return
	}

	loc := getClientLocation(r)

	fromDate, err := time.ParseInLocation("2006-01-02", req.FromDate, loc)
	if err != nil {
		http.Error(w, "Invalid from_date, expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	toDate, err := time.ParseInLocation("2006-01-02", req.ToDate, loc)
	if err != nil {
		http.Error(w, "Invalid to_date, expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	if req.FromDate == req.ToDate {
		http.Error(w, "from_date and to_date must be different", http.StatusBadRequest)
		return
	}

	// Use current time when copying to today; noon on the target date for historical days
	now := time.Now()
	var loggedAt time.Time
	if req.ToDate == now.In(loc).Format("2006-01-02") {
		loggedAt = now.UTC()
	} else {
		loggedAt = time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 12, 0, 0, 0, loc).UTC()
	}

	count, err := db.CopyFoodLogEntries(userID, fromDate, req.MealTags, loggedAt)
	if err != nil {
		slog.Error("failed to copy log entries", "error", err)
		http.Error(w, "Failed to copy log entries", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]int{"count": count}); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}
