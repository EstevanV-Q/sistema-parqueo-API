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

// OpenSQLite abre (o crea) la base local del API de borde y aplica el esquema.
// Usa WAL para permitir lecturas concurrentes y _txlock=immediate para que las
// transacciones de escritura serialicen a los operadores de una misma sede.
func OpenSQLite(path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("SQLITE_PATH es requerido")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolver ruta SQLite: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
		return nil, fmt.Errorf("crear directorio SQLite: %w", err)
	}
	_ = os.Chmod(filepath.Dir(absolute), 0o700)
	if err := ensureSQLiteFilePermissions(absolute); err != nil {
		return nil, fmt.Errorf("asegurar permisos SQLite: %w", err)
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
	if err := ensureRefreshTokenColumns(database); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("migrar refresh_tokens SQLite: %w", err)
	}
	if err := ensureSQLiteFilePermissions(absolute); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("asegurar permisos SQLite: %w", err)
	}
	return database, nil
}

func ensureSQLiteFilePermissions(path string) error {
	for _, name := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(name, 0o600); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func ensureRefreshTokenColumns(database *sql.DB) error {
	rows, err := database.Query(`PRAGMA table_info(refresh_tokens)`)
	if err != nil {
		return err
	}
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			_ = rows.Close()
			return err
		}
		columns[name] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !columns["family_id"] {
		if _, err := database.Exec(`ALTER TABLE refresh_tokens ADD COLUMN family_id BLOB`); err != nil {
			return err
		}
		if _, err := database.Exec(`UPDATE refresh_tokens SET family_id = id WHERE family_id IS NULL`); err != nil {
			return err
		}
	}
	if !columns["replaced_by"] {
		if _, err := database.Exec(`ALTER TABLE refresh_tokens ADD COLUMN replaced_by BLOB`); err != nil {
			return err
		}
	}
	_, err = database.Exec(`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family ON refresh_tokens(family_id)`)
	return err
}

// Open abre la base según el driver configurado. MySQL sigue siendo el modo
// central; SQLite es el modo de borde.
func Open(cfg *config.Config) (*sql.DB, error) {
	if cfg.IsSQLite() {
		return OpenSQLite(cfg.SQLitePath)
	}
	return New(cfg)
}
