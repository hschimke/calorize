package importer

import (
	"os"
	"path/filepath"
	"testing"

	"azule.info/calorize/internal/db"
)

func TestParseFDCFile(t *testing.T) {
	// Make sure we have a dev user or fake user? 
	// The importer does not set creator_id when upserting system foods via StartScanner right now?
	// Ah, food.CreatorID is uuid.Nil which means no creator_id. Let's see if that breaks anything (db might allow it).

	err := parseFDCFile("testdata/fdc_sample.json", nil)
	if err != nil {
		t.Fatalf("parseFDCFile failed: %v", err)
	}

	extID := "fdc_123456"
	baseFood, err := db.GetFoodByExternalID(extID)
	if err != nil {
		t.Fatalf("GetFoodByExternalID failed: %v", err)
	}
	if baseFood == nil {
		t.Fatalf("Expected to find food %s, got nil", extID)
	}

	food, err := db.GetFood(baseFood.ID)
	if err != nil {
		t.Fatalf("GetFood failed: %v", err)
	}

	if food.Name != "Hummus, commercial" {
		t.Errorf("Expected Hummus, got %s", food.Name)
	}
	if food.Calories != 250.0 {
		t.Errorf("Expected 250 calories, got %v", food.Calories)
	}
	if food.Protein != 8.0 {
		t.Errorf("Expected 8.0 protein, got %v", food.Protein)
	}
	if food.Carbs != 14.5 {
		t.Errorf("Expected 14.5 carbs, got %v", food.Carbs)
	}
	if food.Fat != 18.0 {
		t.Errorf("Expected 18.0 fat, got %v", food.Fat)
	}
	
	// Check one micronutrient
	foundC := false
	for _, n := range food.Nutrients {
		if n.Name == "Vitamin C, total ascorbic acid" { // Note: test data has long names usually
			foundC = true
			if n.Amount != 3.2 {
				t.Errorf("Expected 3.2 Vit C, got %v", n.Amount)
			}
		}
	}
	if !foundC {
		t.Errorf("Expected to find Vitamin C in micronutrients")
	}

	// Test Update scenario
	// Parse the same file again and verify it still exists without error, and doesn't explode versions unnecessarily
	err = parseFDCFile("testdata/fdc_sample.json", nil)
	if err != nil {
		t.Fatalf("parseFDCFile second run failed: %v", err)
	}
	
	versions, err := db.GetFoodVersions(food.ID)
	if err != nil {
		t.Fatalf("GetFoodVersions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Errorf("Expected 1 version since data didn't change, got %d", len(versions))
	}

	// Optional: You could mutate the temp file, re-run parseFDCFile, and verify version increases to 2
}

func TestScanner(t *testing.T) {
	importDir := t.TempDir()
	importedDir := t.TempDir()

	// Copy testdata to importDir
	src, err := os.ReadFile("testdata/fdc_sample.json")
	if err != nil {
		t.Fatalf("reading sample: %v", err)
	}
	destPath := filepath.Join(importDir, "import_test.json")
	if err := os.WriteFile(destPath, src, 0644); err != nil {
		t.Fatalf("writing sample to temp: %v", err)
	}

	// Run scanner
	err = ScanAndImport(importDir, importedDir, nil)
	if err != nil {
		t.Fatalf("ScanAndImport failed: %v", err)
	}

	// Verify imported file was moved
	entries, _ := os.ReadDir(importedDir)
	if len(entries) != 1 {
		t.Errorf("Expected 1 file in importedDir, got %d", len(entries))
	} else {
		// Log the name to prove format
		t.Logf("Moved file name: %s", entries[0].Name())
	}

	importEntries, _ := os.ReadDir(importDir)
	if len(importEntries) > 0 {
		t.Errorf("Expected 0 files in importDir, got %d", len(importEntries))
	}
}
