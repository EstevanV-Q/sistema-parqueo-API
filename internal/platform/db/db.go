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

	// 3. Configurar el pool de conexiones (buena práctica de producción)
	db.SetMaxOpenConns(25)                 // Máximo 25 conexiones abiertas simultáneas
	db.SetMaxIdleConns(25)                 // Máximo 25 conexiones inactivas esperando trabajo
	db.SetConnMaxLifetime(5 * time.Minute) // Renovar conexiones cada 5 minutos

	// 4. Probar que realmente haya comunicación con MySQL
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("no se pudo conectar a MySQL: %w", err)
	}

	return db, nil
}
