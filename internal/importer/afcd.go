package importer

import (
	"fmt"
	"log/slog"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"azule.info/calorize/internal/db"
)

// parseUnitFromHeader extracts the unit from a column header like "Calcium (mg)" → "mg".
// Returns "" when no parenthetical unit is present.
func parseUnitFromHeader(header string) string {
	start := strings.LastIndex(header, "(")
	end := strings.LastIndex(header, ")")
	if start == -1 || end == -1 || end <= start+1 {
		return ""
	}
	return strings.TrimSpace(header[start+1 : end])
}

// cellStr safely reads and trims a string from a row slice by column index.
func cellStr(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// normalizeHeader collapses all whitespace (including newlines) into single spaces and trims.
func normalizeHeader(h string) string {
	return strings.Join(strings.Fields(h), " ")
}

// parseGroupInfo reads the "Food group information" sheet and returns
// a map of food group ID → food group name.
// Scans for a header row containing "Food group ID" to handle leading informational rows.
func parseGroupInfo(f *excelize.File) (map[string]string, error) {
	rows, err := f.GetRows("Food group information")
	if err != nil {
		// Fallback to first sheet if named sheet not found
		sheetList := f.GetSheetList()
		if len(sheetList) == 0 {
			return make(map[string]string), nil
		}
		rows, err = f.GetRows(sheetList[0])
		if err != nil {
			return nil, fmt.Errorf("reading food group sheet: %w", err)
		}
	}
	result := make(map[string]string)
	idCol, nameCol := -1, -1
	dataStart := -1
	for i, row := range rows {
		for j, cell := range row {
			norm := normalizeHeader(cell)
			if norm == "Food group ID" {
				idCol = j
			} else if norm == "Food group name" {
				nameCol = j
			}
		}
		if idCol >= 0 && nameCol >= 0 {
			dataStart = i + 1
			break
		}
	}
	if dataStart < 0 {
		return result, nil
	}
	for _, row := range rows[dataStart:] {
		id := cellStr(row, idCol)
		name := cellStr(row, nameCol)
		if id != "" && name != "" {
			result[id] = name
		}
	}
	return result, nil
}

// macroColumns lists Nutrient Profiles columns that are mapped to food fields.
// These are excluded from the food_nutrients (micro) rows.
var macroColumns = map[string]bool{
	"Public Food Key":                                    true,
	"Classification":                                     true,
	"Derivation":                                         true,
	"Food Name":                                          true,
	"Energy with dietary fibre, equated (kJ)":            true,
	"Energy, without dietary fibre, equated (kJ)":        true,
	"Protein (g)":                                        true,
	"Fat, total (g)":                                     true,
	"Available carbohydrates without sugar alcohols (g)": true,
}

type afcdNutrientRow struct {
	EnergyKcal float64
	Protein    float64
	Fat        float64
	Carbs      float64
	Micros     []db.FoodNutrient
}

// parseNutrientProfiles reads the "All solids & liquids per 100 g" sheet and
// returns a map of PublicFoodKey → afcdNutrientRow.
// EnergyKcal is rounded to 2 decimal places to ensure stable float equality on re-import.
func parseNutrientProfiles(f *excelize.File) (map[string]afcdNutrientRow, error) {
	rows, err := f.GetRows("All solids & liquids per 100 g")
	if err != nil {
		return nil, fmt.Errorf("reading nutrient profiles sheet: %w", err)
	}
	if len(rows) < 2 {
		return make(map[string]afcdNutrientRow), nil
	}

	headerRow := -1
	colIdx := make(map[string]int)
	for i, row := range rows {
		for _, cell := range row {
			if normalizeHeader(cell) == "Public Food Key" {
				headerRow = i
				break
			}
		}
		if headerRow >= 0 {
			for j, cell := range rows[headerRow] {
				colIdx[normalizeHeader(cell)] = j
			}
			break
		}
	}

	if headerRow < 0 {
		return nil, fmt.Errorf("could not find header row in nutrient profiles")
	}

	parseF := func(row []string, name string) float64 {
		idx, ok := colIdx[name]
		if !ok {
			return 0
		}
		s := cellStr(row, idx)
		if s == "" {
			return 0
		}
		// Remove commas for numbers like "1,236"
		s = strings.ReplaceAll(s, ",", "")
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			slog.Debug("skipping non-numeric macro cell", "column", name, "value", s)
			return 0
		}
		return v
	}

	headers := rows[headerRow]
	result := make(map[string]afcdNutrientRow, len(rows)-headerRow-1)
	for _, row := range rows[headerRow+1:] {
		key := cellStr(row, colIdx["Public Food Key"])
		if key == "" {
			continue
		}
		energyKJ := parseF(row, "Energy with dietary fibre, equated (kJ)")
		nr := afcdNutrientRow{
			EnergyKcal: math.Round((energyKJ/4.184)*100) / 100,
			Protein:    parseF(row, "Protein (g)"),
			Fat:        parseF(row, "Fat, total (g)"),
			Carbs:      parseF(row, "Available carbohydrates without sugar alcohols (g)"),
		}
		for j, hRaw := range headers {
			h := normalizeHeader(hRaw)
			if macroColumns[h] {
				continue
			}
			s := cellStr(row, j)
			if s == "" {
				continue
			}
			s = strings.ReplaceAll(s, ",", "")
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				slog.Debug("skipping non-numeric nutrient cell", "header", h, "value", s)
				continue
			}
			if v == 0 {
				continue
			}
			nr.Micros = append(nr.Micros, db.FoodNutrient{
				Name:   h,
				Amount: v,
				Unit:   parseUnitFromHeader(h),
			})
		}
		result[key] = nr
	}
	return result, nil
}

// upsertAFCDFood creates or updates the food identified by extID.
// category → Category field, description → IngredientsText field.
// nutrients are inserted fresh under the new version UUID; cascade delete handles cleanup.
func upsertAFCDFood(extID string, food db.Food, category, description string, nutrients []db.FoodNutrient) (upsertResult, error) {
	var catPtr, descPtr *string
	if category != "" {
		catPtr = &category
	}
	if description != "" {
		descPtr = &description
	}
	food.ExternalID = &extID
	food.Category = catPtr
	food.IngredientsText = descPtr
	food.Nutrients = nutrients

	existing, err := db.GetFoodByExternalID(extID)
	if err != nil {
		return upsertSkipped, fmt.Errorf("checking existing food: %w", err)
	}

	if existing != nil {
		existingFull, err := db.GetFood(existing.ID)
		if err != nil {
			return upsertSkipped, fmt.Errorf("fetching full existing food: %w", err)
		}
		if existingFull == nil {
			existingFull = existing
		}
		changed := existing.Name != food.Name ||
			existing.Calories != food.Calories ||
			existing.Protein != food.Protein ||
			existing.Fat != food.Fat ||
			existing.Carbs != food.Carbs ||
			stringPtrChanged(existing.Category, catPtr) ||
			stringPtrChanged(existing.IngredientsText, descPtr) ||
			len(existingFull.Nutrients) != len(nutrients)
		if !changed {
			return upsertSkipped, nil
		}
		if _, err := db.UpdateFood(existing.ID, food); err != nil {
			return upsertSkipped, fmt.Errorf("updating food: %w", err)
		}
		return upsertUpdated, nil
	}

	if _, err := db.CreateFood(food); err != nil {
		return upsertSkipped, fmt.Errorf("creating food: %w", err)
	}
	return upsertCreated, nil
}

// ImportCounts holds tallies from an AFCD import run.
type ImportCounts struct {
	Inserted int
	Updated  int
	Skipped  int
	Errors   int
}

// ImportAFCD reads AFCD Release 3 xlsx files from afcdDir and upserts all foods.
// Expected files (exact names):
//
//	"AFCD Release 3 - Food group information.xlsx"
//	"AFCD Release 3 - Nutrient profiles.xlsx"
//	"AFCD Release 3 - Food Details.xlsx"
func ImportAFCD(afcdDir string) (ImportCounts, error) {
	var counts ImportCounts
	start := time.Now()

	// 1. Food group info → classification code to category name
	fgFile, err := excelize.OpenFile(filepath.Join(afcdDir, "AFCD Release 3 - Food group information.xlsx"))
	if err != nil {
		return counts, fmt.Errorf("opening food group info: %w", err)
	}
	defer fgFile.Close()
	groups, err := parseGroupInfo(fgFile)
	if err != nil {
		return counts, fmt.Errorf("parsing food group info: %w", err)
	}
	slog.Info("AFCD food groups loaded", "count", len(groups))

	// 2. Nutrient profiles → key to nutrient row
	npFile, err := excelize.OpenFile(filepath.Join(afcdDir, "AFCD Release 3 - Nutrient profiles.xlsx"))
	if err != nil {
		return counts, fmt.Errorf("opening nutrient profiles: %w", err)
	}
	defer npFile.Close()
	profiles, err := parseNutrientProfiles(npFile)
	if err != nil {
		return counts, fmt.Errorf("parsing nutrient profiles: %w", err)
	}
	slog.Info("AFCD nutrient profiles loaded", "count", len(profiles))

	// 3. Food details → iterate and upsert
	fdFile, err := excelize.OpenFile(filepath.Join(afcdDir, "AFCD Release 3 - Food Details.xlsx"))
	if err != nil {
		return counts, fmt.Errorf("opening food details: %w", err)
	}
	defer fdFile.Close()

	sheetList := fdFile.GetSheetList()
	if len(sheetList) == 0 {
		return counts, fmt.Errorf("food details file has no sheets")
	}
	// Try "Food details" sheet first, then fallback to first sheet
	sheetName := "Food details"
	rows, err := fdFile.GetRows(sheetName)
	if err != nil {
		sheetName = sheetList[0]
		rows, err = fdFile.GetRows(sheetName)
		if err != nil {
			return counts, fmt.Errorf("reading food details sheet: %w", err)
		}
	}

	if len(rows) < 2 {
		return counts, nil
	}

	headerRow := -1
	colIdx := make(map[string]int)
	for i, row := range rows {
		for _, cell := range row {
			if normalizeHeader(cell) == "Public Food Key" {
				headerRow = i
				break
			}
		}
		if headerRow >= 0 {
			for j, cell := range rows[headerRow] {
				colIdx[normalizeHeader(cell)] = j
			}
			break
		}
	}

	if headerRow < 0 {
		return counts, fmt.Errorf("could not find header row in food details")
	}

	total := 0
	for _, row := range rows[headerRow+1:] {
		key := cellStr(row, colIdx["Public Food Key"])
		if key == "" {
			continue
		}

		total++

		nr, ok := profiles[key]
		if !ok {
			slog.Warn("AFCD food has no nutrient profile, skipping", "key", key)
			counts.Errors++
			continue
		}

		name := cellStr(row, colIdx["Food Name"])
		description := cellStr(row, colIdx["Food Description"])
		classCode := cellStr(row, colIdx["Classification"])
		category := groups[classCode]

		if nr.EnergyKcal == 0 {
			slog.Warn("AFCD food has zero calories", "key", key, "name", name)
		}

		food := db.Food{
			Name:              name,
			Calories:          nr.EnergyKcal,
			Protein:           nr.Protein,
			Fat:               nr.Fat,
			Carbs:             nr.Carbs,
			Type:              "food",
			MeasurementUnit:   "g",
			MeasurementAmount: 100,
			Servings:          1,
			Public:            true,
		}

		result, err := upsertAFCDFood("afcd_"+key, food, category, description, nr.Micros)
		if err != nil {
			slog.Error("AFCD upsert error", "key", key, "name", name, "error", err)
			counts.Errors++
			continue
		}
		switch result {
		case upsertCreated:
			counts.Inserted++
		case upsertUpdated:
			counts.Updated++
		case upsertSkipped:
			counts.Skipped++
		}

		if total%100 == 0 {
			slog.Info("AFCD import progress",
				"processed", total,
				"inserted", counts.Inserted,
				"updated", counts.Updated,
				"skipped", counts.Skipped,
				"errors", counts.Errors,
				"elapsed", time.Since(start).Round(time.Second).String(),
			)
		}
	}

	slog.Info("AFCD import complete",
		"inserted", counts.Inserted,
		"updated", counts.Updated,
		"skipped", counts.Skipped,
		"errors", counts.Errors,
		"elapsed", time.Since(start).Round(time.Second).String(),
	)
	return counts, nil
}
