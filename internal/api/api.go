package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
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
	foods, err := db.GetFoods(userID)
	if err != nil {
		http.Error(w, "Failed to get foods", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(foods)
}

func createFoodHandler(w http.ResponseWriter, r *http.Request) {
	var req createFoodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	var ingredients []db.RecipeItems
	for id, amount := range req.Ingredients {
		foodID, err := uuid.Parse(id)
		if err != nil {
			continue // or handle error
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
		Ingredients:       ingredients,
	})
	if err != nil {
		http.Error(w, "Failed to create food", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(food)
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
	json.NewEncoder(w).Encode(food)
}

func updateFoodHandler(w http.ResponseWriter, r *http.Request) {
	foodIDString := r.PathValue("id")
	foodID, err := uuid.Parse(foodIDString)
	if err != nil {
		http.Error(w, "Invalid food ID", http.StatusBadRequest)
		return
	}
	var req createFoodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	var ingredients []db.RecipeItems
	for id, amount := range req.Ingredients {
		foodID, err := uuid.Parse(id)
		if err != nil {
			continue
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
		Ingredients:       ingredients,
	})
	if err != nil {
		http.Error(w, "Failed to update food", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(food)
}

func deleteFoodHandler(w http.ResponseWriter, r *http.Request) {
	foodIDString := r.PathValue("id")
	foodID, err := uuid.Parse(foodIDString)
	if err != nil {
		http.Error(w, "Invalid food ID", http.StatusBadRequest)
		return
	}
	if err := db.DeleteFood(db.FoodID(foodID)); err != nil {
		slog.Error("failed to delete food", "error", err, "id", foodID)
		http.Error(w, "Failed to delete food", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	date := time.Now()
	if dateStr != "" {
		parsed, err := time.Parse("2006-01-02", dateStr)
		if err == nil {
			date = parsed
		}
	}

	stats, err := db.GetStats(userID, r.URL.Query().Get("period"), date)
	if err != nil {
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
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
	logs, err := db.GetFoodLogEntries(userID, time.Now())
	if err != nil {
		http.Error(w, "Failed to get logs", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

type createLogEntryRequest struct {
	FoodID   db.FoodID `json:"food_id"`
	Amount   float64   `json:"amount"`
	MealTag  string    `json:"meal_tag"`
	LoggedAt time.Time `json:"logged_at"`
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
	entry, err := db.CreateFoodLogEntry(db.FoodLogEntry{UserID: userID, FoodID: req.FoodID, Amount: req.Amount, MealTag: req.MealTag, LoggedAt: req.LoggedAt})
	if err != nil {
		http.Error(w, "Failed to create log entry", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
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
