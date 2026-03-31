package importer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"azule.info/calorize/internal/db"
)

// OFFProduct is a simplified struct for parsing Open Food Facts JSONL exports.
// All nutrient fields are per 100g and stored as grams in OFF (converted on mapping).
type OFFProduct struct {
	Code            string `json:"code"`
	ProductName     string `json:"product_name"`
	Brands          string `json:"brands"`
	Categories      string `json:"categories"`
	IngredientsText string `json:"ingredients_text"`
	ServingSize     string `json:"serving_size"`

	EnergyKcal100g    *float64 `json:"energy-kcal_100g"`
	Proteins100g      *float64 `json:"proteins_100g"`
	Carbohydrates100g *float64 `json:"carbohydrates_100g"`
	Fat100g           *float64 `json:"fat_100g"`

	Sugars100g       *float64 `json:"sugars_100g"`
	Fiber100g        *float64 `json:"fiber_100g"`
	SaturatedFat100g *float64 `json:"saturated-fat_100g"`
	TransFat100g     *float64 `json:"trans-fat_100g"`
	Sodium100g       *float64 `json:"sodium_100g"`
	Cholesterol100g  *float64 `json:"cholesterol_100g"`
	Potassium100g    *float64 `json:"potassium_100g"`
	Calcium100g      *float64 `json:"calcium_100g"`
	Iron100g         *float64 `json:"iron_100g"`
	Magnesium100g    *float64 `json:"magnesium_100g"`
	Zinc100g         *float64 `json:"zinc_100g"`
	// Vitamin C is reliably stored in g/100g across OFF contributors.
	// Vitamins A, D, and B12 are omitted: OFF contributors enter them in mixed units
	// (IU, mcg, or g depending on the source label), making reliable conversion impossible.
	VitaminC100g *float64 `json:"vitamin-c_100g"`
}

// offQualityFilter returns true if the product has enough nutrition data to be useful.
// Requires calories and at least 2 of the 3 core macros.
func offQualityFilter(p OFFProduct) bool {
	if p.ProductName == "" {
		return false
	}
	if p.EnergyKcal100g == nil || *p.EnergyKcal100g == 0 {
		return false
	}
	macroCt := 0
	if p.Proteins100g != nil && *p.Proteins100g > 0 {
		macroCt++
	}
	if p.Carbohydrates100g != nil && *p.Carbohydrates100g > 0 {
		macroCt++
	}
	if p.Fat100g != nil && *p.Fat100g > 0 {
		macroCt++
	}
	return macroCt >= 2
}

// gramWeightRe extracts the first numeric value from a serving_size string (e.g. "30 g", "1 biscuit (30g)").
var gramWeightRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*g`)

// mapOFFToFood converts an OFFProduct to a db.Food, returning the external ID alongside it.
func mapOFFToFood(p OFFProduct) (string, db.Food) {
	extID := "off_" + p.Code

	protein := derefFloat(p.Proteins100g)
	carbs := derefFloat(p.Carbohydrates100g)
	fat := derefFloat(p.Fat100g)

	var brandPtr, barcodePtr, ingredientsPtr, categoryPtr *string
	if brand := strings.TrimSpace(strings.SplitN(p.Brands, ",", 2)[0]); brand != "" {
		brandPtr = &brand
	}
	if p.Code != "" {
		c := p.Code
		barcodePtr = &c
	}
	if p.IngredientsText != "" {
		ingredientsPtr = &p.IngredientsText
	}
	if p.Categories != "" {
		categoryPtr = &p.Categories
	}

	// Parse serving_size for a gram weight portion
	var portions []db.FoodPortion
	if p.ServingSize != "" {
		if m := gramWeightRe.FindStringSubmatch(p.ServingSize); len(m) == 2 {
			if gw, err := strconv.ParseFloat(m[1], 64); err == nil && gw > 0 {
				portions = append(portions, db.FoodPortion{
					Name:       "1 serving",
					Amount:     1,
					GramWeight: gw,
				})
			}
		}
	}

	// Micronutrients — OFF stores all values in g/100g; convert to display units.
	nutrients := mapOFFNutrients(p)

	food := db.Food{
		Name:              p.ProductName,
		Calories:          *p.EnergyKcal100g,
		Protein:           protein,
		Carbs:             carbs,
		Fat:               fat,
		Type:              "food",
		MeasurementUnit:   "g",
		MeasurementAmount: 100,
		Servings:          1,
		Public:            true,
		ExternalID:        &extID,
		BrandOwner:        brandPtr,
		Barcode:           barcodePtr,
		IngredientsText:   ingredientsPtr,
		Category:          categoryPtr,
		Nutrients:         nutrients,
		Portions:          portions,
	}
	return extID, food
}

// mapOFFNutrients converts OFF per-100g gram values to the units used by food_nutrients.
func mapOFFNutrients(p OFFProduct) []db.FoodNutrient {
	type entry struct {
		name   string
		val    *float64
		unit   string
		factor float64 // multiply g/100g value by this to get stored unit
	}
	entries := []entry{
		{"Sugars", p.Sugars100g, "g", 1},
		{"Fiber", p.Fiber100g, "g", 1},
		{"Saturated Fat", p.SaturatedFat100g, "g", 1},
		{"Trans Fat", p.TransFat100g, "g", 1},
		{"Sodium", p.Sodium100g, "mg", 1000},
		{"Cholesterol", p.Cholesterol100g, "mg", 1000},
		{"Potassium", p.Potassium100g, "mg", 1000},
		{"Calcium", p.Calcium100g, "mg", 1000},
		{"Iron", p.Iron100g, "mg", 1000},
		{"Magnesium", p.Magnesium100g, "mg", 1000},
		{"Zinc", p.Zinc100g, "mg", 1000},
		{"Vitamin C", p.VitaminC100g, "mg", 1000},
	}

	var nutrients []db.FoodNutrient
	for _, e := range entries {
		if e.val != nil && *e.val > 0 {
			nutrients = append(nutrients, db.FoodNutrient{
				Name:   e.name,
				Amount: *e.val * e.factor,
				Unit:   e.unit,
			})
		}
	}
	return nutrients
}

func parseOFFFile(filePath string, done <-chan struct{}) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		slog.Warn("could not stat OFF import file", "file", filePath, "error", err)
	} else {
		slog.Info("opening OFF file", "file", filePath, "size_mb", fmt.Sprintf("%.2f", float64(fi.Size())/1e6))
	}
	startTime := time.Now()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024) // 10 MB max line

	var count, errCount, skippedQuality, skippedParse, skippedUnchanged int
	lineNum := 0

	for scanner.Scan() {
		lineNum++

		select {
		case <-done:
			slog.Info("shutdown requested, aborting OFF parse", "processed_so_far", lineNum)
			return fmt.Errorf("aborted mid-parse")
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var product OFFProduct
		if err := json.Unmarshal(line, &product); err != nil {
			skippedParse++
			slog.Warn("OFF: skipping unparseable line", "line", lineNum, "error", err)
			continue
		}

		if !offQualityFilter(product) {
			skippedQuality++
			continue
		}

		extID, foodData := mapOFFToFood(product)
		result, err := upsertFoodData(extID, foodData)
		if err != nil {
			errCount++
			slog.Error("OFF: error upserting food", "code", product.Code, "name", product.ProductName, "error", err)
			continue
		}
		switch result {
		case upsertCreated, upsertUpdated:
			count++
		case upsertSkipped:
			skippedUnchanged++
		}

		total := count + errCount + skippedQuality + skippedParse + skippedUnchanged
		if total%100_000 == 0 && total > 0 {
			slog.Info("OFF import progress",
				"processed", total,
				"inserted_or_updated", count,
				"skipped_unchanged", skippedUnchanged,
				"skipped_quality", skippedQuality,
				"skipped_parse", skippedParse,
				"errors", errCount,
				"elapsed", time.Since(startTime).Round(time.Second).String(),
			)
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("OFF: scanner error", "file", filePath, "error", err)
	}

	slog.Info("finished parsing OFF file",
		"file", filePath,
		"inserted_or_updated", count,
		"skipped_unchanged", skippedUnchanged,
		"skipped_quality", skippedQuality,
		"skipped_parse", skippedParse,
		"errors", errCount,
		"elapsed", time.Since(startTime).Round(time.Second).String(),
	)
	return nil
}

func derefFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}
