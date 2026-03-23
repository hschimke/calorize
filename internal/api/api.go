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
}

func getFoodsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var (
		foods []db.Food
		dbErr error
	)
	if r.URL.Query().Has("recent") {
		foods, dbErr = db.GetRecentFoods(userID, 50)
	} else if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		foods, dbErr = db.SearchFoods(userID, q, 20)
	} else {
		foods, dbErr = db.GetFoods(userID)
	}
	if dbErr != nil {
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

// ### Logs
// - GET /logs
//     - Query Params: ?date=YYYY-MM-DD (Defaults to today)
//     - Returns logs for the day
// - POST /logs
//     - Create log entry
//     - Payload: { food_id, amount, meal_tag, logged_at (optional) }
// - DELETE /logs/{id}

func RegisterLogsPaths(mux *http.ServeMux) {
	mux.HandleFunc("GET /logs", getLogsHandler)
	mux.HandleFunc("POST /logs", createLogEntryHandler)
	mux.HandleFunc("DELETE /logs/{id}", deleteLogEntryHandler)
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
	FoodID   *db.FoodID `json:"food_id"`
	Calories *float64   `json:"calories"`
	Amount   float64    `json:"amount"`
	MealTag  string     `json:"meal_tag"`
	LoggedAt time.Time  `json:"logged_at"`
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

	if req.LoggedAt.IsZero() {
		req.LoggedAt = time.Now().UTC()
	} else {
		req.LoggedAt = req.LoggedAt.UTC()
	}

	entry, err := db.CreateFoodLogEntry(db.FoodLogEntry{
		UserID:   userID,
		FoodID:   req.FoodID,
		Calories: req.Calories,
		Amount:   req.Amount,
		MealTag:  req.MealTag,
		LoggedAt: req.LoggedAt,
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
