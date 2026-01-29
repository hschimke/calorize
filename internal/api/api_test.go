package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"azule.info/calorize/internal/database"
	_ "github.com/mattn/go-sqlite3"
)

// Duplicate setup logic from database test as it is not exported
func setupTestDB(t *testing.T) *sql.DB {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// internal/api -> ../../db/migrations
	migrationsDir := filepath.Join(wd, "..", "..", "db", "migrations")
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		t.Fatalf("migrations directory not found at %s", migrationsDir)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// We can't access database.applyMigrations because it is unexported (lower case)
	// We MUST expose it or duplicate it or just execute sql directly.
	// Since we are adding tests, let's export it in database package for testing purposes?
	// Or just read schema file and exec it manually here for simplicity.

	// Read all migration files
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations dir: %v", err)
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) == ".sql" {
			content, err := os.ReadFile(filepath.Join(migrationsDir, f.Name()))
			if err != nil {
				t.Fatalf("failed to read migration %s: %v", f.Name(), err)
			}
			if _, err := db.Exec(string(content)); err != nil {
				t.Fatalf("failed to apply migration %s: %v", f.Name(), err)
			}
		}
	}

	// Set global DB in database package
	database.DB = db

	return db
}

func TestGetFoods(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert test data
	_, err := db.Exec(`INSERT INTO foods (id, family_id, version, is_current, name, calories, protein, carbs, fat, type, measurement_unit, measurement_amount) 
		VALUES ('1', 'f1', 1, 1, 'Banana', 89, 1.1, 22.8, 0.3, 'food', 'g', 100)`)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	// Create user and session
	user, err := database.CreateUser(context.Background(), "testuser", "test@example.com")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	session, err := database.CreateSession(context.Background(), user.ID, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Create request
	req := httptest.NewRequest("GET", "/foods", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.Token})

	w := httptest.NewRecorder()

	// NewServer initializes WebAuthn, which might fail or panic if config missing.
	// Check internal/auth/webauthn.go first.
	// Assuming it's safe or we need to mock it.
	// If it panics, we'll see.
	handler := NewServer()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var foods []database.Food
	if err := json.NewDecoder(resp.Body).Decode(&foods); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(foods) != 1 {
		t.Errorf("expected 1 food, got %d", len(foods))
	}
	if foods[0].Name != "Banana" {
		t.Errorf("expected food Banana, got %s", foods[0].Name)
	}
}

func TestCreateFood(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create user and session
	user, err := database.CreateUser(context.Background(), "testuser", "test@example.com")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	session, err := database.CreateSession(context.Background(), user.ID, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	f := database.Food{
		Name:              "New Food",
		Calories:          100,
		Protein:           10,
		Carbs:             10,
		Fat:               2,
		Type:              "food",
		MeasurementUnit:   "g",
		MeasurementAmount: 100,
	}
	body, _ := json.Marshal(f)

	req := httptest.NewRequest("POST", "/foods", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.Token})
	// Context?
	// The API uses context from request.

	w := httptest.NewRecorder()
	handler := NewServer()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		// Read body for error
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		t.Errorf("expected status 200, got %d. Body: %s", resp.StatusCode, buf.String())
	}
}
