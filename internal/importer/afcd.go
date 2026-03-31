package importer

import (
	"fmt"
	"math"
	"strconv"
	"strings"

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

// parseGroupInfo reads the first sheet of the Food Group Info xlsx and returns
// a map of food group ID → food group name.
// Scans for a header row containing "Food group ID" to handle leading informational rows.
func parseGroupInfo(f *excelize.File) (map[string]string, error) {
	sheetList := f.GetSheetList()
	if len(sheetList) == 0 {
		return make(map[string]string), nil
	}
	rows, err := f.GetRows(sheetList[0])
	if err != nil {
		return nil, fmt.Errorf("reading food group sheet: %w", err)
	}
	result := make(map[string]string)
	idCol, nameCol := -1, -1
	dataStart := -1
	for i, row := range rows {
		for j, cell := range row {
			switch strings.TrimSpace(cell) {
			case "Food group ID":
				idCol = j
			case "Food group name":
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

	headers := rows[0]
	colIdx := make(map[string]int, len(headers))
	for i, h := range headers {
		colIdx[strings.TrimSpace(h)] = i
	}

	parseF := func(row []string, name string) float64 {
		s := cellStr(row, colIdx[name])
		if s == "" {
			return 0
		}
		v, _ := strconv.ParseFloat(s, 64)
		return v
	}

	result := make(map[string]afcdNutrientRow, len(rows)-1)
	for _, row := range rows[1:] {
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
		for i, h := range headers {
			h = strings.TrimSpace(h)
			if macroColumns[h] {
				continue
			}
			s := cellStr(row, i)
			if s == "" {
				continue
			}
			v, err := strconv.ParseFloat(s, 64)
			if err != nil || v == 0 {
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
