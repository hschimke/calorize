package importer

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"azule.info/calorize/internal/db"
)

func TestParseUnitFromHeader(t *testing.T) {
	cases := []struct{ header, want string }{
		{"Calcium (mg)", "mg"},
		{"Energy with dietary fibre, equated (kJ)", "kJ"},
		{"Protein (g)", "g"},
		{"Public Food Key", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := parseUnitFromHeader(c.header)
		if got != c.want {
			t.Errorf("parseUnitFromHeader(%q) = %q, want %q", c.header, got, c.want)
		}
	}
}

func TestParseGroupInfo(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()

	// Row 1: informational noise (like the real AFCD file)
	f.SetCellValue("Sheet1", "A1", "Some general note about this database")
	// Row 2: actual column headers
	f.SetCellValue("Sheet1", "A2", "Food group ID")
	f.SetCellValue("Sheet1", "B2", "Food group name")
	f.SetCellValue("Sheet1", "C2", "Inclusions")
	// Data rows
	f.SetCellValue("Sheet1", "A3", "7")
	f.SetCellValue("Sheet1", "B3", "Vegetable products and dishes")
	f.SetCellValue("Sheet1", "A4", "31")
	f.SetCellValue("Sheet1", "B4", "Cereal-based products and dishes")

	groups, err := parseGroupInfo(f)
	if err != nil {
		t.Fatalf("parseGroupInfo: %v", err)
	}
	if got := groups["7"]; got != "Vegetable products and dishes" {
		t.Errorf("groups[7] = %q, want %q", got, "Vegetable products and dishes")
	}
	if got := groups["31"]; got != "Cereal-based products and dishes" {
		t.Errorf("groups[31] = %q, want %q", got, "Cereal-based products and dishes")
	}
	if len(groups) != 2 {
		t.Errorf("len(groups) = %d, want 2", len(groups))
	}
}

func TestParseNutrientProfiles(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "All solids & liquids per 100 g"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	headers := []string{
		"Public Food Key",
		"Classification",
		"Derivation",
		"Food Name",
		"Energy with dietary fibre, equated (kJ)",
		"Energy, without dietary fibre, equated (kJ)",
		"Protein (g)",
		"Fat, total (g)",
		"Available carbohydrates without sugar alcohols (g)",
		"Calcium (mg)",
		"Moisture (water) (g)",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	values := []interface{}{
		"F000001", "7", "Analysed", "Carrot, raw",
		169.0, // kJ → rounded kcal
		152.0, // kJ without fibre (macro column, excluded from micros)
		0.9,   // protein
		0.2,   // fat
		7.3,   // carbs
		33.0,  // calcium → micro
		88.2,  // moisture → micro
	}
	for i, v := range values {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(sheet, cell, v)
	}

	profiles, err := parseNutrientProfiles(f)
	if err != nil {
		t.Fatalf("parseNutrientProfiles: %v", err)
	}
	row, ok := profiles["F000001"]
	if !ok {
		t.Fatal("expected F000001 in profiles")
	}

	wantKcal := math.Round((169.0/4.184)*100) / 100
	if row.EnergyKcal != wantKcal {
		t.Errorf("EnergyKcal = %.4f, want %.4f", row.EnergyKcal, wantKcal)
	}
	if row.Protein != 0.9 {
		t.Errorf("Protein = %v, want 0.9", row.Protein)
	}
	if row.Fat != 0.2 {
		t.Errorf("Fat = %v, want 0.2", row.Fat)
	}
	if row.Carbs != 7.3 {
		t.Errorf("Carbs = %v, want 7.3", row.Carbs)
	}
	// Calcium + Moisture = 2 micros (all macro columns excluded)
	if len(row.Micros) != 2 {
		t.Errorf("len(Micros) = %d, want 2 (Calcium + Moisture)", len(row.Micros))
	}
	var foundCalcium bool
	for _, m := range row.Micros {
		if m.Name == "Calcium (mg)" && m.Unit == "mg" && m.Amount == 33.0 {
			foundCalcium = true
		}
	}
	if !foundCalcium {
		t.Error("expected Calcium (mg) micronutrient with amount=33 and unit=mg")
	}
}

func TestUpsertAFCDFood(t *testing.T) {
	// Unique ID per test run to avoid state from previous runs
	extID := fmt.Sprintf("afcd_UPSERT_%d", time.Now().UnixNano())
	food := db.Food{
		Name:              "Carrot test, raw",
		Calories:          40.4,
		Protein:           0.9,
		Carbs:             7.3,
		Fat:               0.2,
		Type:              "food",
		MeasurementUnit:   "g",
		MeasurementAmount: 100,
		Servings:          1,
		Public:            true,
	}
	nutrients := []db.FoodNutrient{
		{Name: "Calcium (mg)", Amount: 33.0, Unit: "mg"},
	}

	// First call → created
	result, err := upsertAFCDFood(extID, food, "Vegetables", "A test carrot", nutrients)
	if err != nil {
		t.Fatalf("upsertAFCDFood: %v", err)
	}
	if result != upsertCreated {
		t.Errorf("first call: got %v, want upsertCreated", result)
	}

	got, err := db.GetFoodByExternalID(extID)
	if err != nil || got == nil {
		t.Fatalf("food not found after insert: %v", err)
	}
	full, err := db.GetFood(got.ID)
	if err != nil {
		t.Fatalf("GetFood: %v", err)
	}
	if full.Name != "Carrot test, raw" {
		t.Errorf("Name = %q, want %q", full.Name, "Carrot test, raw")
	}
	if len(full.Nutrients) != 1 {
		t.Errorf("len(Nutrients) = %d, want 1", len(full.Nutrients))
	}
	if full.Category == nil || *full.Category != "Vegetables" {
		t.Errorf("Category = %v, want Vegetables", full.Category)
	}

	// Same data again → skipped, version count stays at 1
	result2, err := upsertAFCDFood(extID, food, "Vegetables", "A test carrot", nutrients)
	if err != nil {
		t.Fatalf("second upsertAFCDFood: %v", err)
	}
	if result2 != upsertSkipped {
		t.Errorf("second call: got %v, want upsertSkipped", result2)
	}
	versions, _ := db.GetFoodVersions(got.ID)
	if len(versions) != 1 {
		t.Errorf("versions = %d, want 1 (no change detected)", len(versions))
	}

	// Changed data → updated, version count becomes 2
	food.Name = "Carrot test, cooked"
	result3, err := upsertAFCDFood(extID, food, "Vegetables", "A test carrot", nutrients)
	if err != nil {
		t.Fatalf("third upsertAFCDFood: %v", err)
	}
	if result3 != upsertUpdated {
		t.Errorf("third call (changed name): got %v, want upsertUpdated", result3)
	}
	updated, _ := db.GetFoodByExternalID(extID)
	versionsAfterUpdate, _ := db.GetFoodVersions(updated.ID)
	if len(versionsAfterUpdate) != 2 {
		t.Errorf("versions after update = %d, want 2", len(versionsAfterUpdate))
	}
}
