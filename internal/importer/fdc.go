package importer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

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

	fi, err := file.Stat()
	if err != nil {
		slog.Warn("could not stat import file", "file", filePath, "error", err)
	} else {
		slog.Info("opening FDC file", "file", filePath, "size_mb", fmt.Sprintf("%.2f", float64(fi.Size())/1e6))
	}
	startTime := time.Now()

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

	var count, errCount, skippedCount int
	for decoder.More() {
		select {
		case <-done:
			slog.Info("shutdown requested, aborting FDC parse", "processed_so_far", count)
			return fmt.Errorf("aborted mid-parse")
		default:
		}

		var fdcFood FdcFood
		err := decoder.Decode(&fdcFood)
		if err != nil {
			var typeErr *json.UnmarshalTypeError
			if errors.As(err, &typeErr) {
				// Field type mismatch: decoder consumed the full token cleanly, safe to skip
				errCount++
				slog.Warn("skipping FDC record with type mismatch", "index", count+errCount+skippedCount, "field", typeErr.Field, "error", err)
				continue
			}
			// Syntax error or I/O error: stream position is undefined, cannot recover
			return fmt.Errorf("unrecoverable decode error at index %d: %w", count+errCount+skippedCount, err)
		}

		result, err := upsertFood(fdcFood)
		if err != nil {
			errCount++
			slog.Error("error upserting food", "fdcId", fdcFood.FdcID, "description", fdcFood.Description, "error", err)
			continue
		}

		switch result {
		case upsertSkipped:
			skippedCount++
		case upsertCreated, upsertUpdated:
			count++
		}

		total := count + errCount + skippedCount
		if total%1000 == 0 {
			slog.Info("FDC import progress",
				"processed", total,
				"inserted_or_updated", count,
				"skipped_unchanged", skippedCount,
				"errors", errCount,
				"elapsed", time.Since(startTime).Round(time.Second).String(),
			)
		}
	}

	slog.Info("finished parsing FDC file",
		"file", filePath,
		"inserted_or_updated", count,
		"skipped_unchanged", skippedCount,
		"errors", errCount,
		"elapsed", time.Since(startTime).Round(time.Second).String(),
	)
	return nil
}

type upsertResult int

const (
	upsertCreated upsertResult = iota
	upsertUpdated
	upsertSkipped
)

func upsertFood(fdcFood FdcFood) (upsertResult, error) {
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
	var fatSource string
	if fatLipid > 0 {
		fat = fatLipid
		fatSource = "lipid"
	} else if fatNlea > 0 {
		fat = fatNlea
		fatSource = "nlea"
	} else {
		fatSource = "none"
	}

	// Resolve Carbs (priority: difference > summation > calculation from components)
	var carbSource string
	if carbDiff > 0 {
		carbs = carbDiff
		carbSource = "by_difference"
	} else if carbSum > 0 {
		carbs = carbSum
		carbSource = "by_summation"
	} else {
		carbs = carbSugars + carbStarch + carbFiber
		carbSource = "components"
	}

	// Resolve Energy (priority: kcal > specific > general > kJ > macro estimation)
	var calSource string
	if calKcal > 0 {
		calories = calKcal
		calSource = "kcal"
	} else if calAtwaterSpecific > 0 {
		calories = calAtwaterSpecific
		calSource = "atwater_specific"
	} else if calAtwaterGeneral > 0 {
		calories = calAtwaterGeneral
		calSource = "atwater_general"
	} else if calKj > 0 {
		calories = calKj / 4.184
		calSource = "kj_converted"
	} else {
		// Fallback: estimation from macros
		// This helps with Foundation Foods that might only provide raw components.
		calories = (protein * 4) + (carbs * 4) + (fat * 9)
		calSource = "macro_estimate"
	}

	// Log unusual resolution paths
	if calSource == "kj_converted" || calSource == "macro_estimate" {
		slog.Warn("unusual calorie source", "fdcId", fdcFood.FdcID, "description", fdcFood.Description, "cal_source", calSource, "calories", calories)
	} else if calSource != "kcal" {
		slog.Debug("non-kcal calorie source", "fdcId", fdcFood.FdcID, "description", fdcFood.Description, "cal_source", calSource, "calories", calories)
	}
	if carbSource == "components" {
		slog.Warn("carb fallback to components", "fdcId", fdcFood.FdcID, "description", fdcFood.Description, "sugars", carbSugars, "starch", carbStarch, "fiber", carbFiber, "total", carbs)
	}
	if fatSource == "nlea" {
		slog.Debug("fat resolved via NLEA", "fdcId", fdcFood.FdcID, "description", fdcFood.Description)
	}
	if fatSource == "none" {
		slog.Warn("no fat data found", "fdcId", fdcFood.FdcID, "description", fdcFood.Description)
	}
	if calories == 0 {
		slog.Warn("zero calories after resolution", "fdcId", fdcFood.FdcID, "description", fdcFood.Description, "cal_source", calSource, "protein", protein, "carbs", carbs, "fat", fat)
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
		return upsertSkipped, fmt.Errorf("checking existing food: %w", err)
	}

	if existingFood != nil {
		// Only update if macros or name changed to avoid db bloat on every startup run.
		if existingFood.Calories != calories || existingFood.Protein != protein || existingFood.Carbs != carbs || existingFood.Fat != fat || existingFood.Name != fdcFood.Description {
			slog.Debug("updating changed food", "fdcId", fdcFood.FdcID, "description", fdcFood.Description,
				"old_cal", existingFood.Calories, "new_cal", calories,
				"old_protein", existingFood.Protein, "new_protein", protein,
				"old_carbs", existingFood.Carbs, "new_carbs", carbs,
				"old_fat", existingFood.Fat, "new_fat", fat,
			)
			_, err = db.UpdateFood(existingFood.ID, foodData)
			if err != nil {
				return upsertSkipped, fmt.Errorf("updating food: %w", err)
			}
			return upsertUpdated, nil
		}
		return upsertSkipped, nil
	}

	_, err = db.CreateFood(foodData)
	if err != nil {
		return upsertSkipped, fmt.Errorf("creating food: %w", err)
	}
	return upsertCreated, nil
}
