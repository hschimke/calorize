package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS
var db *sql.DB

func init() {
	goose.SetBaseFS(embedMigrations)

	var err error

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./test.db"
	}

	dsn := fmt.Sprintf(
		"%s?_pragma=foreign_keys(1)"+
			"&_pragma=journal_mode(WAL)"+
			"&_pragma=busy_timeout(5000)"+
			"&_pragma=synchronous(NORMAL)"+   // safe with WAL, faster than FULL
			"&_pragma=cache_size(-64000)"+    // 64 MB page cache
			"&_pragma=temp_store(MEMORY)"+    // temp tables in RAM
			"&_pragma=mmap_size(134217728)"+  // 128 MB memory-mapped I/O
			"&_pragma=analysis_limit(400)",   // sample limit for ANALYZE / optimize
		dbPath,
	)

	slog.Info("opening database", "path", dsn)
	db, err = sql.Open("sqlite", dsn)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		panic(err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(5 * time.Minute)

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

// Close runs PRAGMA optimize to refresh query planner statistics, then closes
// the underlying database connection pool. It should be called once during
// application shutdown.
func Close() error {
	if db != nil {
		// Update stale index statistics before closing so the next startup
		// benefits from accurate query plans.
		if _, err := db.Exec("PRAGMA optimize;"); err != nil {
			slog.Warn("PRAGMA optimize failed", "error", err)
		}
		return db.Close()
	}
	return nil
}
