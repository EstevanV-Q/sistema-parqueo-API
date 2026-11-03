package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"

	"parking-backend/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: go run cmd/migrate/main.go <up|down|status>")
		os.Exit(1)
	}
	command := os.Args[1]

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Configuración inválida: %v\n", err)
		os.Exit(1)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
		cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Printf("Error al abrir DB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Printf("Error al conectar a MySQL: %v\n", err)
		os.Exit(1)
	}

	goose.SetDialect("mysql")

	switch command {
	case "up":
		if err := goose.Up(db, "migrations"); err != nil {
			fmt.Printf("Error al migrar: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migraciones aplicadas correctamente")
	case "down":
		if err := goose.Down(db, "migrations"); err != nil {
			fmt.Printf("Error al revertir: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Última migración revertida")
	case "status":
		if err := goose.Status(db, "migrations"); err != nil {
			fmt.Printf("Error al obtener estado: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Comando desconocido: %s\n", command)
		os.Exit(1)
	}
}
