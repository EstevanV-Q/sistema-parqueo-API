// Comando dbcheck prueba la conexión a la base configurada en .env y verifica
// que el esquema mínimo esté aplicado. No imprime contraseñas.
//
// Uso:
//
//	go run ./cmd/dbcheck
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"time"

	"parking-backend/internal/config"
	platformdb "parking-backend/internal/platform/db"
)

var requiredTables = []string{
	"audit_log",
	"customers",
	"device_events",
	"devices",
	"idempotency_keys",
	"payments",
	"rate_plans",
	"refresh_tokens",
	"sites",
	"spaces",
	"subscriptions",
	"ticket_events",
	"tickets",
	"users",
	"vehicle_types",
	"vehicles",
	"zones",
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuración inválida: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var database *sql.DB
	if cfg.IsSQLite() {
		fmt.Printf("Driver: sqlite\nArchivo: %s\n\n", cfg.SQLitePath)
		database, err = platformdb.Open(cfg)
	} else {
		fmt.Printf("Driver: mysql\nHost: %s:%s\nBase de datos: %s\nUsuario: %s\nTLS: %s\n\n",
			cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBUser, cfg.DBTLS)
		database, err = platformdb.Open(cfg)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR de conexión: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	if err := database.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR de conexión: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Conexión OK")

	if cfg.IsSQLite() {
		var version string
		_ = database.QueryRowContext(ctx, `SELECT sqlite_version()`).Scan(&version)
		fmt.Printf("SQLite: %s\n", version)
	} else {
		var version, currentDB, currentUser, hostname string
		err := database.QueryRowContext(ctx,
			`SELECT VERSION(), DATABASE(), CURRENT_USER(), @@hostname`).
			Scan(&version, &currentDB, &currentUser, &hostname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR consultando el servidor: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("MySQL: %s\nBase actual: %s\nUsuario actual: %s\nHost: %s\n", version, currentDB, currentUser, hostname)
	}

	tables, err := listTables(ctx, database, cfg.IsSQLite())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR listando tablas: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nTablas encontradas: %d\n", len(tables))

	missing := make([]string, 0)
	required := append([]string{}, requiredTables...)
	if !cfg.IsSQLite() {
		required = append(required, "sync_inbox")
	}
	for _, table := range required {
		if !tables[table] {
			missing = append(missing, table)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		fmt.Fprintf(os.Stderr, "\nFALTAN TABLAS DEL ESQUEMA: %v\n", missing)
		fmt.Fprintln(os.Stderr, "Ejecuta las migraciones: go run ./cmd/migrate/main.go up")
		os.Exit(1)
	}
	fmt.Println("Esquema mínimo: OK")
}

func listTables(ctx context.Context, database *sql.DB, sqlite bool) (map[string]bool, error) {
	query := `SHOW TABLES`
	if sqlite {
		query = `SELECT name FROM sqlite_master WHERE type = 'table'`
	}
	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	tables := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables[name] = true
	}
	return tables, rows.Err()
}
