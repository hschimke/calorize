# AFCD Importer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone `afcd-importer` binary that reads AFCD Release 3 xlsx files and imports ~1,589 Australian foods (with full micronutrient data) into the Calorize SQLite database.

**Architecture:** New `internal/importer/afcd.go` with three sheet parsers and an idempotent upsert function, mirroring the existing FDC importer pattern. A thin `cmd/afcd-importer/main.go` CLI wires env vars to the importer. The Docker image gains the second binary.

**Tech Stack:** Go 1.26, `github.com/xuri/excelize/v2` (xlsx), existing `internal/db` package (SQLite via glebarez/go-sqlite), `log/slog`.

---

## File Map

| Action | Path | Purpose |
|---|---|---|
| Create | `internal/importer/afcd.go` | All AFCD parsing + import logic |
| Create | `internal/importer/afcd_test.go` | Tests for afcd.go |
| Create | `cmd/afcd-importer/main.go` | CLI entry point |
| Modify | `go.mod` / `go.sum` | Add excelize dependency |
| Modify | `docker/Dockerfile.api` | Build + copy afcd-importer binary |

**Key existing files to understand (read-only):**
- `internal/importer/fdc.go` — defines `upsertResult`, `upsertCreated`, `upsertUpdated`, `upsertSkipped`, `stringPtrChanged` in package `importer`; all reusable by afcd.go
- `internal/db/model.go` — `db.Food`, `db.FoodNutrient` structs
- `internal/db/database.go` — db auto-initialises via `init()` from `DB_PATH` env var (defaults to `./test.db`). `init()` runs at package load time, **before** any test code, so `DB_PATH` cannot be changed by a TestMain. Tests must use unique keys per run (see Task 5) to stay idempotent.
- `docker/Dockerfile.api` — multi-stage build; line 15 has the api-server build; final stage copies binaries and `chown`s them to `appuser:appgroup`

---

## Task 1: Add excelize dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/xuri/excelize/v2
```

- [ ] **Step 2: Verify**

```bash
grep excelize go.mod
```
Expected: line containing `github.com/xuri/excelize/v2`

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add excelize dependency for xlsx parsing"
```

---

## Task 2: parseUnitFromHeader — TDD

**Files:**
- Create: `internal/importer/afcd.go`
- Create: `internal/importer/afcd_test.go`

> **Import note:** Add only the imports used at each step. The skeleton at this step needs only `"strings"`. Remaining imports are added in subsequent tasks as functions that require them are implemented.

- [ ] **Step 1: Write the failing test** in `internal/importer/afcd_test.go`:

```go
package importer

import (
	"testing"
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
```

- [ ] **Step 2: Run to confirm FAIL**

```bash
go test ./internal/importer/ -run TestParseUnitFromHeader
```
Expected: `undefined: parseUnitFromHeader`

- [ ] **Step 3: Create `internal/importer/afcd.go`**

```go
package importer

import (
	"strings"
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
```

- [ ] **Step 4: Run to confirm PASS**

```bash
go test ./internal/importer/ -run TestParseUnitFromHeader
```
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/importer/afcd.go internal/importer/afcd_test.go
git commit -m "feat: add afcd.go skeleton with parseUnitFromHeader"
```

---

## Task 3: parseGroupInfo — TDD

**Files:**
- Modify: `internal/importer/afcd.go`
- Modify: `internal/importer/afcd_test.go`

The Food Group Info xlsx may have informational rows above the real header. `parseGroupInfo` scans for the row containing "Food group ID" to find the data start. It uses `f.GetSheetList()[0]` (first sheet) to avoid hardcoding the sheet name.

- [ ] **Step 1: Write the failing test** (append to `afcd_test.go`):

```go
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
```

Also add the excelize import to `afcd_test.go`:
```go
import (
	"testing"
	"github.com/xuri/excelize/v2"
)
```

- [ ] **Step 2: Run to confirm FAIL**

```bash
go test ./internal/importer/ -run TestParseGroupInfo
```
Expected: `undefined: parseGroupInfo`

- [ ] **Step 3: Implement `parseGroupInfo`** — add to `afcd.go`, expanding imports to `"fmt"`, `"strings"`, `"github.com/xuri/excelize/v2"`:

```go
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
```

- [ ] **Step 4: Run to confirm PASS**

```bash
go test ./internal/importer/ -run TestParseGroupInfo
```
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/importer/afcd.go internal/importer/afcd_test.go
git commit -m "feat: implement parseGroupInfo for AFCD category lookup"
```

---

## Task 4: parseNutrientProfiles — TDD

**Files:**
- Modify: `internal/importer/afcd.go`
- Modify: `internal/importer/afcd_test.go`

The Nutrient Profiles sheet ("All solids & liquids per 100 g") has a header row then one row per food. Macro columns are extracted to `afcdNutrientRow` fields; all other non-zero columns become `Micros`. Energy is in kJ, divided by 4.184. Calories are rounded to 2 decimal places before storage to ensure float equality holds on re-import.

- [ ] **Step 1: Write the failing test** (append to `afcd_test.go`):

```go
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
```

Add `"math"` to the test file's imports.

- [ ] **Step 2: Run to confirm FAIL**

```bash
go test ./internal/importer/ -run TestParseNutrientProfiles
```
Expected: `undefined: parseNutrientProfiles`

- [ ] **Step 3: Implement `afcdNutrientRow` and `parseNutrientProfiles`** — add to `afcd.go`, expanding imports to include `"math"`, `"strconv"`, `"azule.info/calorize/internal/db"`:

```go
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
```

- [ ] **Step 4: Run to confirm PASS**

```bash
go test ./internal/importer/ -run TestParseNutrientProfiles
```
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/importer/afcd.go internal/importer/afcd_test.go
git commit -m "feat: implement parseNutrientProfiles for AFCD nutrient data"
```

---

## Task 5: upsertAFCDFood — TDD

**Files:**
- Modify: `internal/importer/afcd.go`
- Modify: `internal/importer/afcd_test.go`

`upsertResult`, `upsertCreated`, `upsertUpdated`, `upsertSkipped`, and `stringPtrChanged` are already defined in `fdc.go` (same package). Do not redefine them.

> **Test isolation note:** Because `db.init()` runs at package load time (before any test code), `DB_PATH` cannot be overridden per-test. Tests use time-based unique external IDs so each `go test` run creates fresh records, ensuring assertions are valid on repeated runs.

- [ ] **Step 1: Write the failing test** (append to `afcd_test.go`):

```go
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
```

Add `"fmt"`, `"time"`, `"azule.info/calorize/internal/db"` to `afcd_test.go` imports.

- [ ] **Step 2: Run to confirm FAIL**

```bash
go test ./internal/importer/ -run TestUpsertAFCDFood
```
Expected: `undefined: upsertAFCDFood`

- [ ] **Step 3: Implement `upsertAFCDFood`** — add to `afcd.go`:

```go
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
		changed := existing.Name != food.Name ||
			existing.Calories != food.Calories ||
			existing.Protein != food.Protein ||
			existing.Fat != food.Fat ||
			existing.Carbs != food.Carbs ||
			stringPtrChanged(existing.Category, catPtr) ||
			stringPtrChanged(existing.IngredientsText, descPtr)
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
```

Also add `"fmt"` to `afcd.go` imports if not already present.

- [ ] **Step 4: Run to confirm PASS**

```bash
go test ./internal/importer/ -run TestUpsertAFCDFood
```
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/importer/afcd.go internal/importer/afcd_test.go
git commit -m "feat: implement upsertAFCDFood with idempotent upsert pattern"
```

---

## Task 6: ImportAFCD (main entry point) — TDD

**Files:**
- Modify: `internal/importer/afcd.go`
- Modify: `internal/importer/afcd_test.go`

Reads three xlsx files from a directory, joins them, and calls `upsertAFCDFood` per food. Uses `f.GetSheetList()[0]` for the Food Details sheet to handle any sheet name.

- [ ] **Step 1: Write the failing test** (append to `afcd_test.go`):

```go
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
```

Add `"path/filepath"` to `afcd_test.go` imports.

- [ ] **Step 2: Run to confirm FAIL**

```bash
go test ./internal/importer/ -run TestImportAFCD
```
Expected: `undefined: ImportAFCD`

- [ ] **Step 3: Implement `ImportCounts` and `ImportAFCD`** — add to `afcd.go`, expanding imports to include `"log/slog"`, `"path/filepath"`, `"time"`:

```go
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
	rows, err := fdFile.GetRows(sheetList[0])
	if err != nil {
		return counts, fmt.Errorf("reading food details sheet: %w", err)
	}
	if len(rows) < 2 {
		return counts, nil
	}

	colIdx := make(map[string]int, len(rows[0]))
	for i, h := range rows[0] {
		colIdx[strings.TrimSpace(h)] = i
	}

	total := 0
	for _, row := range rows[1:] {
		key := cellStr(row, colIdx["Public Food Key"])
		if key == "" {
			continue
		}

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

		total++
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
```

- [ ] **Step 4: Run all AFCD tests**

```bash
go test ./internal/importer/ -run "TestParseUnit|TestParseGroup|TestParseNutrient|TestUpsertAFCD|TestImportAFCD" -v
```
Expected: all `PASS`

- [ ] **Step 5: Run full importer test suite (including FDC tests) to check for regressions**

```bash
go test ./internal/importer/ -v
```
Expected: all `PASS`

- [ ] **Step 6: Commit**

```bash
git add internal/importer/afcd.go internal/importer/afcd_test.go
git commit -m "feat: implement ImportAFCD with progress logging and idempotent upsert"
```

---

## Task 7: cmd/afcd-importer/main.go

**Files:**
- Create: `cmd/afcd-importer/main.go`

No test needed — thin CLI wrapper. The `db.init()` runs automatically via the transitive import of `internal/db` through `internal/importer`. No explicit db init call needed.

- [ ] **Step 1: Create `cmd/afcd-importer/main.go`**

```go
package main

import (
	"log/slog"
	"os"

	// Importing importer triggers db.init() via the transitive db import.
	"azule.info/calorize/internal/importer"
)

func main() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	afcdDir := os.Getenv("AFCD_DIR")
	if afcdDir == "" {
		afcdDir = "./aus"
	}

	slog.Info("AFCD importer starting",
		"afcd_dir", afcdDir,
		"db_path", os.Getenv("DB_PATH"),
	)

	counts, err := importer.ImportAFCD(afcdDir)
	if err != nil {
		slog.Error("import failed", "error", err)
		os.Exit(1)
	}

	slog.Info("import finished",
		"inserted", counts.Inserted,
		"updated", counts.Updated,
		"skipped", counts.Skipped,
		"errors", counts.Errors,
	)
	if counts.Errors > 0 {
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./cmd/afcd-importer/
```
Expected: no errors

- [ ] **Step 3: Clean up local binary**

```bash
rm -f ./afcd-importer
```

- [ ] **Step 4: Commit**

```bash
git add cmd/afcd-importer/main.go
git commit -m "feat: add afcd-importer CLI entry point"
```

---

## Task 8: Update Dockerfile

**Files:**
- Modify: `docker/Dockerfile.api`

Read the file before editing. The builder stage has the `go build` for api-server on line 15. The final stage copies binaries and `chown`s them to `appuser:appgroup`.

- [ ] **Step 1: Add build line to builder stage** — after the existing api-server `go build` line:

```dockerfile
RUN CGO_ENABLED=0 GOOS=linux go build -o afcd-importer ./cmd/afcd-importer/
```

- [ ] **Step 2: Add copy + chown to final stage** — after the existing `COPY --from=builder /app/api-server .` and its `chown` line:

```dockerfile
COPY --from=builder /app/afcd-importer .
RUN chown appuser:appgroup ./afcd-importer
```

- [ ] **Step 3: Verify Docker build succeeds**

```bash
docker build -f docker/Dockerfile.api -t calorize-api-test . && echo "BUILD OK"
```
Expected: `BUILD OK`

- [ ] **Step 4: Verify the binary is in the image**

```bash
docker run --rm calorize-api-test ls -la /app/afcd-importer
```
Expected: file owned by `appuser`

- [ ] **Step 5: Clean up test image**

```bash
docker rmi calorize-api-test
```

- [ ] **Step 6: Commit**

```bash
git add docker/Dockerfile.api
git commit -m "feat: include afcd-importer binary in Docker image"
```

---

## End-to-End Verification

After all tasks complete, verify against the real AFCD files:

```bash
# Local dev (uses ./aus/ by default)
DB_PATH=./verify_test.db go run ./cmd/afcd-importer/
```

Expected log output includes:
- `AFCD food groups loaded count=102` (approx)
- `AFCD nutrient profiles loaded count=1590` (approx)
- `AFCD import complete inserted=1589 updated=0 skipped=0 errors=0` (approx)

Spot-check queries:
```bash
# Count imported foods
sqlite3 ./verify_test.db "SELECT count(*) FROM foods WHERE external_id LIKE 'afcd_%';"
# → ~1589

# Spot-check one food's macros and category
sqlite3 ./verify_test.db "SELECT name, calories, protein, carbs, fat, category FROM foods WHERE external_id = 'afcd_F002258';"

# Count total micronutrient rows
sqlite3 ./verify_test.db "SELECT count(*) FROM food_nutrients fn JOIN foods f ON f.id = fn.food_id WHERE f.external_id LIKE 'afcd_%';"
# → large number (many nutrients per food)

# Re-run: all skipped (idempotency check)
DB_PATH=./verify_test.db go run ./cmd/afcd-importer/
# → inserted=0 updated=0 skipped=~1589 errors=0

# Clean up
rm -f ./verify_test.db
```

**Docker verification:**
```bash
# Place xlsx files in $MAPDIR/afcd/ on the host
docker-compose -f docker/docker-compose.yml run --rm -e AFCD_DIR=/data/afcd calorize-api /app/afcd-importer
# Check DB on host at $MAPDIR/calorize.db
sqlite3 "$MAPDIR/calorize.db" "SELECT count(*) FROM foods WHERE external_id LIKE 'afcd_%';"
```
