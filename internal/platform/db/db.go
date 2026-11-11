package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql" // El guión bajo (_) es para registrar el driver sin usar funciones directas
	"parking-backend/internal/config"
)

// New crea y configura el pool de conexiones a MySQL
func New(cfg *config.Config) (*sql.DB, error) {
	// 1. Armar la cadena de conexión DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	// 2. Abrir el pool
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("error al abrir conexión: %w", err)
	}

	// 3. Configurar el pool de conexiones según la configuración de la aplicación.
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

	// 4. Probar que realmente haya comunicación con MySQL
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("no se pudo conectar a MySQL: %w", err)
	}

	return db, nil
}
