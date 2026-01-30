package db

import (
	"database/sql"
	"embed"
	"log/slog"

	_ "github.com/glebarez/go-sqlite"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS
var db *sql.DB

func init() {
	goose.SetBaseFS(embedMigrations)

	var err error

	slog.Info("opening database", "path", "./test.db?_fk=1&_journal=WAL&_busy_timeout=5000")
	db, err = sql.Open("sqlite", "file:./test.db?_fk=1&_journal=WAL&_busy_timeout=5000")
	if err != nil {
		slog.Error("failed to open database", "error", err)
		panic(err)
	}

	if err := goose.SetDialect("sqlite3"); err != nil {
		slog.Error("failed to set dialect", "error", err)
		panic(err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		slog.Error("failed to run migrations", "error", err)
		panic(err)
	}

	slog.Info("database initialized")
}
