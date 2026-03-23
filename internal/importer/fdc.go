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
	var calKcal, calAtwaterGeneral, calAtwaterSpecific, calKj float64
	var carbDiff, carbSum, carbSugars, carbStarch, carbFiber float64
	var fatLipid, fatNlea float64
	var microNutrients []db.FoodNutrient

	for _, fn := range fdcFood.FoodNutrients {
		// FDC Numbers:
		// 208, 1008 = Energy (kcal)
		// 957 = Energy (Atwater General Factors)
		// 958 = Energy (Atwater Specific Factors)
		// 268, 1062 = Energy (kJ)
		// 203, 1003 = Protein
		// 205, 1005 = Carbohydrate, by difference
		// 205.2, 1050 = Carbohydrate, by summation
		// 269, 269.3, 2000, 1063 = Sugars, Total
		// 209, 1009 = Starch
		// 291, 1079 = Fiber, total dietary
		// 204, 1004 = Total lipid (fat)
		// 298, 1085 = Total fat (NLEA)
		switch fn.Nutrient.Number {
		case "208", "1008": // kcal
			calKcal = fn.Amount
		case "957": // Atwater General Factors
			calAtwaterGeneral = fn.Amount
		case "958": // Atwater Specific Factors
			calAtwaterSpecific = fn.Amount
		case "268", "1062": // kJ
			calKj = fn.Amount
		case "203", "1003":
			protein = fn.Amount
		case "205", "1005":
			carbDiff = fn.Amount
		case "205.2", "1050":
			carbSum = fn.Amount
		case "269", "269.3", "2000", "1063":
			carbSugars = fn.Amount
		case "209", "1009":
			carbStarch = fn.Amount
		case "291", "1079":
			carbFiber = fn.Amount
		case "204", "1004":
			fatLipid = fn.Amount
		case "298", "1085":
			fatNlea = fn.Amount
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

	// Resolve Fat (priority: lipid > nlea)
	if fatLipid > 0 {
		fat = fatLipid
	} else {
		fat = fatNlea
	}

	// Resolve Carbs (priority: difference > summation > calculation from components)
	if carbDiff > 0 {
		carbs = carbDiff
	} else if carbSum > 0 {
		carbs = carbSum
	} else {
		// Fallback to components
		carbs = carbSugars + carbStarch + carbFiber
	}

	// Resolve Energy (priority: kcal > specific > general > kJ > macro estimation)
	if calKcal > 0 {
		calories = calKcal
	} else if calAtwaterSpecific > 0 {
		calories = calAtwaterSpecific
	} else if calAtwaterGeneral > 0 {
		calories = calAtwaterGeneral
	} else if calKj > 0 {
		calories = calKj / 4.184
	} else {
		// Fallback: estimation from macros
		// This helps with Foundation Foods that might only provide raw components.
		calories = (protein * 4) + (carbs * 4) + (fat * 9)
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
