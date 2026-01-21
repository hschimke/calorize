package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB(dbPath string) error {
	var err error

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Determine migrations directory
	// In production/deployment, this might need to be configurable
	// For now, assuming relative path to the binary or current working directory
	migrationsDir := filepath.Join(filepath.Dir(dbPath), "migrations")

	// Fallback to "db/migrations" if relative to CWD
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		migrationsDir = "db/migrations"
	}

	return applyMigrations(DB, migrationsDir)
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}
