package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql" // registra el driver
	"parking-backend/internal/config"
)

// New abre el pool de conexiones a MySQL y verifica la conexión.
func New(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("error al abrir conexión: %w", err)
	}

	maxOpen := cfg.DBMaxOpen
	if maxOpen <= 0 {
		maxOpen = 25
	}
	maxIdle := cfg.DBMaxIdle
	if maxIdle <= 0 {
		maxIdle = 25
	}
	maxLife := cfg.DBMaxLife
	if maxLife <= 0 {
		maxLife = 5 * time.Minute
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLife)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("no se pudo conectar a MySQL: %w", err)
	}

	return db, nil
}
