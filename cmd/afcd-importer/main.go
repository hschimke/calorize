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
