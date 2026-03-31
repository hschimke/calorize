package importer

import (
	"fmt"
	"math"
	"path/filepath"
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

func TestImportAFCD(t *testing.T) {
	dir := t.TempDir()

	// Unique food key per run for test isolation
	testKey := fmt.Sprintf("F_IMP_%d", time.Now().UnixNano())
	expectedExtID := "afcd_" + testKey

	// --- Food group information ---
	fg := excelize.NewFile()
	defer fg.Close()
	fg.SetCellValue("Sheet1", "A1", "Food group ID")
	fg.SetCellValue("Sheet1", "B1", "Food group name")
	fg.SetCellValue("Sheet1", "A2", "7")
	fg.SetCellValue("Sheet1", "B2", "Vegetable products and dishes")
	if err := fg.SaveAs(filepath.Join(dir, "AFCD Release 3 - Food group information.xlsx")); err != nil {
		t.Fatal(err)
	}

	// --- Nutrient profiles ---
	np := excelize.NewFile()
	defer np.Close()
	npSheet := "All solids & liquids per 100 g"
	np.NewSheet(npSheet)
	np.DeleteSheet("Sheet1")
	npHeaders := []string{
		"Public Food Key", "Classification", "Derivation", "Food Name",
		"Energy with dietary fibre, equated (kJ)", "Protein (g)",
		"Fat, total (g)", "Available carbohydrates without sugar alcohols (g)",
		"Calcium (mg)",
	}
	for i, h := range npHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		np.SetCellValue(npSheet, cell, h)
	}
	npVals := []interface{}{testKey, "7", "Analysed", "Import Test Food", 169.0, 0.9, 0.2, 7.3, 33.0}
	for i, v := range npVals {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		np.SetCellValue(npSheet, cell, v)
	}
	if err := np.SaveAs(filepath.Join(dir, "AFCD Release 3 - Nutrient profiles.xlsx")); err != nil {
		t.Fatal(err)
	}

	// --- Food details ---
	fd := excelize.NewFile()
	defer fd.Close()
	fd.SetCellValue("Sheet1", "A1", "Public Food Key")
	fd.SetCellValue("Sheet1", "B1", "Classification")
	fd.SetCellValue("Sheet1", "C1", "Food Name")
	fd.SetCellValue("Sheet1", "D1", "Food Description")
	fd.SetCellValue("Sheet1", "A2", testKey)
	fd.SetCellValue("Sheet1", "B2", "7")
	fd.SetCellValue("Sheet1", "C2", "Import Test Food")
	fd.SetCellValue("Sheet1", "D2", "A food for testing the importer")
	if err := fd.SaveAs(filepath.Join(dir, "AFCD Release 3 - Food Details.xlsx")); err != nil {
		t.Fatal(err)
	}

	counts, err := ImportAFCD(dir)
	if err != nil {
		t.Fatalf("ImportAFCD: %v", err)
	}
	if counts.Inserted != 1 {
		t.Errorf("Inserted = %d, want 1", counts.Inserted)
	}
	if counts.Errors != 0 {
		t.Errorf("Errors = %d, want 0", counts.Errors)
	}

	food, err := db.GetFoodByExternalID(expectedExtID)
	if err != nil || food == nil {
		t.Fatalf("food %q not found after import: %v", expectedExtID, err)
	}
	full, _ := db.GetFood(food.ID)
	if full.Category == nil || *full.Category != "Vegetable products and dishes" {
		t.Errorf("Category = %v, want 'Vegetable products and dishes'", full.Category)
	}
	if len(full.Nutrients) != 1 {
		t.Errorf("len(Nutrients) = %d, want 1 (Calcium)", len(full.Nutrients))
	}

	// Re-run same files → all skipped
	counts2, err := ImportAFCD(dir)
	if err != nil {
		t.Fatalf("ImportAFCD second run: %v", err)
	}
	if counts2.Inserted != 0 || counts2.Skipped != 1 {
		t.Errorf("second run: Inserted=%d Skipped=%d, want Inserted=0 Skipped=1",
			counts2.Inserted, counts2.Skipped)
	}
}
