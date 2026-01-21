package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	// Create a temp file for the database
	// using :memory: might be faster but sometimes has issues with shared cache if we open multiple connections?
	// database.go opens a connection.
	// Let's use a temp file to be safe and closer to reality
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// We need to find the migrations directory.
	// Since we are in internal/database, it should be ../../db/migrations
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Traverse up until we find go.mod or hit root?
	// Or just hardcode relatively for now.
	migrationsDir := filepath.Join(wd, "..", "..", "db", "migrations")
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		t.Fatalf("migrations directory not found at %s", migrationsDir)
	}

	// Use InitDB from the package?
	// InitDB takes a path.
	// However, InitDB replaces the global DB variable.
	// That's fine for tests if we run them sequentially or if we accept that.
	// But `go test` runs packages in parallel usually. Within a package, tests are sequential unless t.Parallel() is called.
	// Start with serial.

	// We can manually init to avoid side effects if we wanted, but the code uses the global DB variable.
	// So we MUST set the global DB variable.

	// However, InitDB logic is:
	// mkdir
	// sql.Open
	// Ping
	// applyMigrations

	// We can reuse InitDB, but we need to trick it to find migrations.
	// But InitDB looks for migrations relative to the dbPath or CWD.
	// If we pass `dir/test.db`, it looks in `dir/migrations`.
	// So we might need to copy migrations there OR just mock it.

	// Actually, let's just do it manually here to have control.

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := applyMigrations(db, migrationsDir); err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	// SET GLOBAL DB
	DB = db

	return db
}

func teardownTestDB(t *testing.T) {
	if DB != nil {
		DB.Close()
		DB = nil
	}
}

func TestMain(m *testing.M) {
	// Global setup if needed
	os.Exit(m.Run())
}
