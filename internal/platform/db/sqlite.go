package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"parking-backend/internal/config"
)

//go:embed sqlite_schema.sql
var sqliteSchema string

// OpenSQLite abre (o crea) el archivo SQLite y aplica el esquema.
// WAL permite leer mientras se escribe; _txlock=immediate serializa escrituras.
func OpenSQLite(path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("SQLITE_PATH es requerido")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolver ruta SQLite: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o750); err != nil {
		return nil, fmt.Errorf("crear directorio SQLite: %w", err)
	}

	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)&_txlock=immediate",
		filepath.ToSlash(absolute),
	)
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("abrir SQLite: %w", err)
	}
	database.SetMaxOpenConns(4)
	database.SetMaxIdleConns(4)
	database.SetConnMaxLifetime(0)

	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("conectar a SQLite: %w", err)
	}
	if _, err := database.Exec(sqliteSchema); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("aplicar esquema SQLite: %w", err)
	}
	return database, nil
}

// Open abre la base según DB_DRIVER: MySQL (servidor) o SQLite (archivo local).
func Open(cfg *config.Config) (*sql.DB, error) {
	if cfg.IsSQLite() {
		return OpenSQLite(cfg.SQLitePath)
	}
	return New(cfg)
}
