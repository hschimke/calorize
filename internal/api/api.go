package api

import (
	"encoding/json"
	"net/http"
	"time"

	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/db"
	"github.com/google/uuid"
)

func RegisterApiPaths(mux *http.ServeMux) {
	RegisterLogsPaths(mux)
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
	foods, err := db.GetFoods(db.UserID(r.Context().Value(auth.UserIDContextKey).(uuid.UUID)))
	if err != nil {
		http.Error(w, "Failed to get foods", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(foods)
}

func createFoodHandler(w http.ResponseWriter, r *http.Request) {
	var req createFoodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	db.CreateFood(db.Food{CreatorID: db.UserID(r.Context().Value(auth.UserIDContextKey).(uuid.UUID)), Name: req.Name, Calories: req.Calories, Protein: req.Protein, Carbs: req.Carbs, Fat: req.Fat, Type: req.Type, MeasurementUnit: req.MeasurementUnit, MeasurementAmount: req.MeasurementAmount})
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
	db.UpdateFood(db.FoodID(foodID), db.Food{CreatorID: db.UserID(r.Context().Value(auth.UserIDContextKey).(uuid.UUID)), Name: req.Name, Calories: req.Calories, Protein: req.Protein, Carbs: req.Carbs, Fat: req.Fat, Type: req.Type, MeasurementUnit: req.MeasurementUnit, MeasurementAmount: req.MeasurementAmount})
}

func deleteFoodHandler(w http.ResponseWriter, r *http.Request) {
	foodIDString := r.PathValue("id")
	foodID, err := uuid.Parse(foodIDString)
	if err != nil {
		http.Error(w, "Invalid food ID", http.StatusBadRequest)
		return
	}
	db.DeleteFood(db.FoodID(foodID))
}

// ### Stats
// - GET /stats
//     - Query Params: ?period={day,week,month}&date=YYYY-MM-DD
//     - Returns aggregated macros and total calories

func RegisterStatsPaths(mux *http.ServeMux) {
	mux.HandleFunc("GET /stats", getStatsHandler)
}

func getStatsHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := db.GetStats(db.UserID(r.Context().Value(auth.UserIDContextKey).(uuid.UUID)), r.URL.Query().Get("period"), time.Now())
	if err != nil {
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}
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
	db.GetFoodLogEntries(db.UserID(r.Context().Value(auth.UserIDContextKey).(uuid.UUID)), time.Now())
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
	db.CreateFoodLogEntry(db.FoodLogEntry{UserID: db.UserID(r.Context().Value(auth.UserIDContextKey).(uuid.UUID)), FoodID: req.FoodID, Amount: req.Amount, MealTag: req.MealTag, LoggedAt: req.LoggedAt})
}

func deleteLogEntryHandler(w http.ResponseWriter, r *http.Request) {
	logEntryIdString := r.PathValue("id")
	logEntryId, err := uuid.Parse(logEntryIdString)
	if err != nil {
		http.Error(w, "Invalid log entry ID", http.StatusBadRequest)
		return
	}
	db.DeleteFoodLogEntry(db.FoodLogEntryID(logEntryId), db.UserID(r.Context().Value(auth.UserIDContextKey).(uuid.UUID)))
}
