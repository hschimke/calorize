package api

import (
	"encoding/json"
	"net/http"

	"azule.info/calorize/internal/database"
)

func GetFoods(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	foods, err := database.GetFoods(ctx, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(foods)
}

type CreateFoodRequest struct {
	Name              string              `json:"name"`
	Calories          float64             `json:"calories"`
	Protein           float64             `json:"protein"`
	Carbs             float64             `json:"carbs"`
	Fat               float64             `json:"fat"`
	Type              string              `json:"type"` // 'food' or 'recipe'
	MeasurementUnit   string              `json:"measurement_unit"`
	MeasurementAmount float64             `json:"measurement_amount"`
	Ingredients       map[string]float64  `json:"ingredients"` // If type=recipe
	FamilyID          string              `json:"family_id"`   // Optional, if updating
	Nutrients         []database.Nutrient `json:"nutrients"`
}

func CreateFood(w http.ResponseWriter, r *http.Request) {
	var req CreateFoodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	food := &database.Food{
		Name:              req.Name,
		Calories:          req.Calories,
		Protein:           req.Protein,
		Carbs:             req.Carbs,
		Fat:               req.Fat,
		Type:              req.Type,
		MeasurementUnit:   req.MeasurementUnit,
		MeasurementAmount: req.MeasurementAmount,
		FamilyID:          req.FamilyID,
		Nutrients:         req.Nutrients,
	}

	if food.Type == "" {
		food.Type = "food"
	}

	ctx := r.Context()
	if err := database.CreateFood(ctx, food); err != nil {
		http.Error(w, "failed to create food: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if req.Type == "recipe" && len(req.Ingredients) > 0 {
		if err := database.AddRecipeItems(ctx, food.ID, req.Ingredients); err != nil {
			// Warn?
		}
	}

	json.NewEncoder(w).Encode(food)
}

type IngredientResponse struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

type FoodResponse struct {
	*database.Food
	Ingredients []IngredientResponse `json:"ingredients,omitempty"`
}

func GetFood(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	food, err := database.GetFood(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if food == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	res := FoodResponse{Food: food}

	// Fetch ingredients if recipe
	if food.Type == "recipe" {
		items, err := database.GetRecipeItems(ctx, food.ID)
		if err != nil {
			// Log error but maybe return partial?
			// For now, fail hard to debug
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, item := range items {
			res.Ingredients = append(res.Ingredients, IngredientResponse{
				ID:     item.Food.ID,
				Name:   item.Food.Name,
				Amount: item.Amount,
			})
		}
	}
	json.NewEncoder(w).Encode(res)
}

func UpdateFood(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	var req CreateFoodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// 1. Look up existing food to get Family ID
	existing, err := database.GetFood(ctx, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// 2. Create new version using existing Family ID
	food := &database.Food{
		Name:              req.Name,
		Calories:          req.Calories,
		Protein:           req.Protein,
		Carbs:             req.Carbs,
		Fat:               req.Fat,
		Type:              req.Type,
		MeasurementUnit:   req.MeasurementUnit,
		MeasurementAmount: req.MeasurementAmount,
		FamilyID:          existing.FamilyID, // Enforce Family Continuity
		Nutrients:         req.Nutrients,
	}
	if food.Type == "" {
		food.Type = existing.Type // Default to existing type
	}

	if err := database.CreateFood(ctx, food); err != nil {
		http.Error(w, "failed to update food: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if (req.Type == "recipe" || food.Type == "recipe") && len(req.Ingredients) > 0 {
		if err := database.AddRecipeItems(ctx, food.ID, req.Ingredients); err != nil {
			// Warn
		}
	}

	json.NewEncoder(w).Encode(food)
}

func DeleteFood(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	if err := database.DeleteFood(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
