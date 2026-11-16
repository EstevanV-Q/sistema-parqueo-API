// Comando seed: carga los datos de demostración (MySQL o SQLite).
// Las contraseñas se leen de variables de entorno, nunca del código.
package main

import (
	"log"

	"parking-backend/internal/config"
	platformdb "parking-backend/internal/platform/db"
	"parking-backend/internal/seed"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuración inválida: %v", err)
	}
	database, err := platformdb.Open(cfg)
	if err != nil {
		log.Fatalf("no se pudo abrir la base de datos: %v", err)
	}
	defer func() { _ = database.Close() }()

	res, err := seed.Run(database, cfg)
	if err != nil {
		log.Fatalf("seed falló: %v", err)
	}
	log.Printf("Seed completado. Sede: %s (%s)", res.SiteName, res.SiteID)
	log.Printf("Usuarios: admin, operador, auditor. Abonado de prueba: %s", cfg.SeedVehiclePlate)
}
