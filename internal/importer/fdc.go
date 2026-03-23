package importer

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"azule.info/calorize/internal/db"
)

// FdcFood is a simplified struct for parsing the huge JSON
type FdcFood struct {
	FdcID         int           `json:"fdcId"`
	Description   string        `json:"description"`
	FoodNutrients []FdcNutrient `json:"foodNutrients"`
}

type FdcNutrient struct {
	Nutrient FdcNutrientInfo `json:"nutrient"`
	Amount   float64         `json:"amount"`
}

type FdcNutrientInfo struct {
	ID       int    `json:"id"`
	Number   string `json:"number"`
	Name     string `json:"name"`
	UnitName string `json:"unitName"`
}

func parseFDCFile(filePath string, done <-chan struct{}) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	// We don't know the exact top-level key yet (might be "BrandedFoods" or "FoundationFoods")
	// so we manually seek until we find the start of the array.
	var arrayName string
	for {
		t, err := decoder.Token()
		if err == io.EOF {
			return fmt.Errorf("no array found in json")
		}
		if err != nil {
			return fmt.Errorf("decoder token error: %w", err)
		}

		if str, ok := t.(string); ok && (str == "BrandedFoods" || str == "FoundationFoods") {
			arrayName = str
			slog.Info("found target array key", "key", arrayName)
			continue
		}

		if delim, ok := t.(json.Delim); ok && delim == '[' && arrayName != "" {
			// Found the start of the array
			break
		}
	}

	count := 0
	for decoder.More() {
		select {
		case <-done:
			slog.Info("shutdown requested, aborting FDC parse")
			return fmt.Errorf("aborted mid-parse")
		default:
		}

		var fdcFood FdcFood
		err := decoder.Decode(&fdcFood)
		if err != nil {
			return fmt.Errorf("error decoding object at idx %d: %w", count, err)
		}

		err = upsertFood(fdcFood)
		if err != nil {
			slog.Error("error upserting food", "fdcId", fdcFood.FdcID, "error", err)
			continue
		}

		count++
		if count%1000 == 0 {
			slog.Info("processed FDC records", "count", count)
		}
	}

	slog.Info("finished parsing FDC file", "total_records", count)
	return nil
}

func upsertFood(fdcFood FdcFood) error {
	extID := fmt.Sprintf("fdc_%d", fdcFood.FdcID)

	// Map nutrients
	var calories, protein, carbs, fat float64
	var microNutrients []db.FoodNutrient

	for _, fn := range fdcFood.FoodNutrients {
		// FDC Numbers:
		// 208 = Energy (Calories)
		// 203 = Protein
		// 205 = Carbohydrate, by difference
		// 204 = Total lipid (fat)
		switch fn.Nutrient.Number {
		case "208", "1008": // kcal
			calories = fn.Amount
		case "203", "1003":
			protein = fn.Amount
		case "205", "1005":
			carbs = fn.Amount
		case "204", "1004":
			fat = fn.Amount
		default:
			if fn.Amount > 0 {
				microNutrients = append(microNutrients, db.FoodNutrient{
					Name:   fn.Nutrient.Name,
					Amount: fn.Amount,
					Unit:   fn.Nutrient.UnitName,
				})
			}
		}
	}

	foodData := db.Food{
		Name:              fdcFood.Description,
		Calories:          calories,
		Protein:           protein,
		Carbs:             carbs,
		Fat:               fat,
		Type:              "food",
		MeasurementUnit:   "g",
		MeasurementAmount: 100,
		Servings:          1,
		Public:            true,
		ExternalID:        &extID,
		Nutrients:         microNutrients,
	}

	existingFood, err := db.GetFoodByExternalID(extID)
	if err != nil {
		return fmt.Errorf("checking existing food: %w", err)
	}

	if existingFood != nil {
		// We could compare properties here, but for simplicity, we'll
		// update if anything fundamental has changed, or just unconditionally update.
		// Actually, to prevent huge db bloat on every startup run, let's only do it if the macros changed.
		if existingFood.Calories != calories || existingFood.Protein != protein || existingFood.Carbs != carbs || existingFood.Fat != fat || existingFood.Name != fdcFood.Description {
			_, err = db.UpdateFood(existingFood.ID, foodData)
			if err != nil {
				return fmt.Errorf("updating food: %w", err)
			}
		}
	} else {
		_, err = db.CreateFood(foodData)
		if err != nil {
			return fmt.Errorf("creating food: %w", err)
		}
	}

	return nil
}
