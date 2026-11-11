// Comando seed carga datos de demostración de forma idempotente.
// Todos los secretos se leen de variables de entorno (vía config.Load).
// Nunca escribir contraseñas o claves directamente en el código.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"parking-backend/internal/config"
	"parking-backend/internal/edge"
	platformdb "parking-backend/internal/platform/db"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/platform/security"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuración inválida: %v", err)
	}
	security.ConfigureTokenPepper(cfg.TokenPepper)
	if cfg.Env == "production" && !cfg.AllowSeed {
		log.Fatal("el seed está bloqueado en producción; define ALLOW_SEED=true solo si es imprescindible")
	}
	if cfg.SeedAdminPassword == "" || cfg.SeedOperatorPassword == "" || cfg.SeedAuditorPassword == "" {
		log.Fatal("SEED_ADMIN_PASSWORD, SEED_OPERATOR_PASSWORD y SEED_AUDITOR_PASSWORD son requeridos para ejecutar el seed")
	}
	for name, password := range map[string]string{
		"SEED_ADMIN_PASSWORD":    cfg.SeedAdminPassword,
		"SEED_OPERATOR_PASSWORD": cfg.SeedOperatorPassword,
		"SEED_AUDITOR_PASSWORD":  cfg.SeedAuditorPassword,
	} {
		if err := security.ValidateSeedPassword(password); err != nil {
			log.Fatalf("%s: %v", name, err)
		}
	}
	if err := security.ValidateSeedPassword(cfg.SeedDeviceAPIKey); err != nil {
		log.Fatalf("SEED_DEVICE_API_KEY: %v", err)
	}
	if len(cfg.SeedDeviceAPIKey) < 16 {
		log.Fatal("SEED_DEVICE_API_KEY debe tener al menos 16 caracteres")
	}

	if cfg.IsSQLite() {
		database, err := platformdb.Open(cfg)
		if err != nil {
			log.Fatalf("Error al abrir SQLite: %v", err)
		}
		defer func() { _ = database.Close() }()
		result, err := edge.Seed(database, cfg)
		if err != nil {
			log.Fatalf("Error al poblar el borde: %v", err)
		}
		log.Println("✅ Seed de borde completado")
		log.Printf("📍 Sede: %s (ID: %s)", result.SiteName, result.SiteID)
		log.Printf("🚗 Vehículo abonado: %s", cfg.SeedVehiclePlate)
		log.Printf("🚧 Dispositivo: %s", result.DeviceID)
		log.Printf("   IDs: admin=%s operador=%s auditor=%s", result.AdminID, result.OperatorID, result.AuditorID)
		log.Println("   Usuarios: admin, operador, auditor (contraseñas definidas por variables de entorno)")
		return
	}

	dsn, err := platformdb.MySQLDSN(cfg)
	if err != nil {
		log.Fatalf("Configuración de base de datos inválida: %v", err)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Error al abrir DB: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Ping(); err != nil {
		log.Fatalf("Error al conectar a MySQL: %v", err)
	}

	now := time.Now().UTC()

	// 1. Sede
	siteID := insertIgnoreAndGet(db,
		`INSERT IGNORE INTO sites (id, name, code, timezone, currency, capacity_total, is_active, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, TRUE, ?)`,
		[]any{ids.StringToBin16(ids.UUIDv7String()), cfg.SeedSiteName, cfg.SeedSiteCode, cfg.SiteTimezone, "COP", 50, now},
		`SELECT id FROM sites WHERE code = ?`, cfg.SeedSiteCode,
	)
	if siteID == "" {
		log.Fatal("No se pudo obtener o crear la sede")
	}
	siteBin := ids.StringToBin16(siteID)

	// 2. Usuarios (contraseñas desde variables de entorno)
	adminHash, err := security.HashPassword(cfg.SeedAdminPassword)
	if err != nil {
		log.Fatalf("Error hasheando contraseña admin: %v", err)
	}
	adminID := insertIgnoreAndGet(db,
		`INSERT IGNORE INTO users (id, username, email, password_hash, role, site_id, active, created_at)
		 VALUES (?, ?, ?, ?, ?, NULL, TRUE, ?)`,
		[]any{ids.StringToBin16(ids.UUIDv7String()), "admin", "admin@parqueo.local", adminHash, "admin", now},
		`SELECT id FROM users WHERE username = ?`, "admin",
	)

	operHash, err := security.HashPassword(cfg.SeedOperatorPassword)
	if err != nil {
		log.Fatalf("Error hasheando contraseña operador: %v", err)
	}
	operID := insertIgnoreAndGet(db,
		`INSERT IGNORE INTO users (id, username, email, password_hash, role, site_id, active, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, TRUE, ?)`,
		[]any{ids.StringToBin16(ids.UUIDv7String()), "operador", "operador@parqueo.local", operHash, "operator", siteBin, now},
		`SELECT id FROM users WHERE username = ?`, "operador",
	)
	// Si el operador existía sin sede, se le asigna la sede demo.
	if _, err := db.Exec(`UPDATE users SET site_id = ? WHERE username = ?`, siteBin, "operador"); err != nil {
		log.Printf("Aviso: no se pudo asignar sede al operador: %v", err)
	}

	auditorHash, err := security.HashPassword(cfg.SeedAuditorPassword)
	if err != nil {
		log.Fatalf("Error hasheando contraseña auditor: %v", err)
	}
	auditorID := insertIgnoreAndGet(db,
		`INSERT IGNORE INTO users (id, username, email, password_hash, role, site_id, active, created_at)
		 VALUES (?, ?, ?, ?, ?, NULL, TRUE, ?)`,
		[]any{ids.StringToBin16(ids.UUIDv7String()), "auditor", "auditor@parqueo.local", auditorHash, "auditor", now},
		`SELECT id FROM users WHERE username = ?`, "auditor",
	)

	// El seed es determinista: si los usuarios ya existían, rota sus
	// credenciales a las definidas por las variables de entorno.
	usersToUpdate := []struct {
		id    string
		hash  string
		email string
		role  string
		site  any
	}{
		{adminID, adminHash, "admin@parqueo.local", "admin", nil},
		{operID, operHash, "operador@parqueo.local", "operator", siteBin},
		{auditorID, auditorHash, "auditor@parqueo.local", "auditor", nil},
	}
	for _, u := range usersToUpdate {
		if u.id == "" {
			continue
		}
		if _, err := db.Exec(
			`UPDATE users SET password_hash=?, email=?, role=?, site_id=?, active=TRUE WHERE id=?`,
			u.hash, u.email, u.role, u.site, ids.StringToBin16(u.id),
		); err != nil {
			log.Fatalf("No se pudo actualizar el usuario %s: %v", u.email, err)
		}
	}

	// 3. Zona
	zoneID := queryID(db, `SELECT id FROM zones WHERE site_id = ? AND name = ? LIMIT 1`, siteBin, "General - Carros")
	if zoneID == "" {
		zoneID = ids.UUIDv7String()
		if _, err := db.Exec(`INSERT INTO zones (id, site_id, name, type, capacity, is_active, created_at)
			VALUES (?, ?, ?, ?, ?, TRUE, ?)`,
			ids.StringToBin16(zoneID), siteBin, "General - Carros", "car", 30, now); err != nil {
			log.Printf("Error al crear zona: %v\n", err)
		}
	}
	zoneBin := ids.StringToBin16(zoneID)

	// 4. Espacios (10 carros)
	for i := 1; i <= 10; i++ {
		code := fmt.Sprintf("A-%02d", i)
		if _, err := db.Exec(`INSERT IGNORE INTO spaces (id, zone_id, code, type, status, is_active, created_at)
			VALUES (?, ?, ?, ?, 'free', TRUE, ?)`,
			ids.StringToBin16(ids.UUIDv7String()), zoneBin, code, "car", now); err != nil {
			log.Printf("Aviso en espacio %s: %v\n", code, err)
		}
	}

	// 5. Tipo de vehículo
	vetypeID := insertIgnoreAndGet(db,
		`INSERT IGNORE INTO vehicle_types (id, name, code, is_active, created_at)
		 VALUES (?, ?, ?, TRUE, ?)`,
		[]any{ids.StringToBin16(ids.UUIDv7String()), "Carro", "car", now},
		`SELECT id FROM vehicle_types WHERE code = ?`, "car",
	)
	vetypeBin := ids.StringToBin16(vetypeID)

	// 6. Tarifa
	rateID := queryID(db, `SELECT id FROM rate_plans WHERE site_id = ? AND name = ? LIMIT 1`, siteBin, "Estándar Carro")
	if rateID == "" {
		rateID = ids.UUIDv7String()
		if _, err := db.Exec(`INSERT INTO rate_plans
			(id, site_id, name, vehicle_type_id, currency, grace_minutes, minimum_cents, fraction_minutes, fraction_cents, daily_cap_cents, lost_ticket_cents, is_active, valid_from, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?)`,
			ids.StringToBin16(rateID), siteBin, "Estándar Carro", vetypeBin, "COP", 10, 1000, 60, 1500, 20000, 25000, now, now); err != nil {
			log.Printf("Error al crear tarifa: %v\n", err)
		}
	}

	// 7. Cliente
	custID := insertIgnoreAndGet(db,
		`INSERT IGNORE INTO customers (id, document, name, phone, email, is_active, created_at)
		 VALUES (?, ?, ?, ?, ?, TRUE, ?)`,
		[]any{ids.StringToBin16(ids.UUIDv7String()), "12345678", "Juan Pérez", "3001234567", "juan@email.com", now},
		`SELECT id FROM customers WHERE document = ?`, "12345678",
	)
	custBin := ids.StringToBin16(custID)

	// 8. Vehículo
	plateNorm := cfg.SeedVehiclePlate
	vehID := insertIgnoreAndGet(db,
		`INSERT IGNORE INTO vehicles (id, plate_normalized, plate_raw, type_id, customer_id, is_active, created_at)
		 VALUES (?, ?, ?, ?, ?, TRUE, ?)`,
		[]any{ids.StringToBin16(ids.UUIDv7String()), plateNorm, "XYZ-987", vetypeBin, custBin, now},
		`SELECT id FROM vehicles WHERE plate_normalized = ?`, plateNorm,
	)
	vehBin := ids.StringToBin16(vehID)

	// 9. Abonado vigente (30 días)
	subID := queryID(db, `SELECT id FROM subscriptions WHERE vehicle_id = ? AND status = 'active' LIMIT 1`, vehBin)
	if subID == "" {
		subID = ids.UUIDv7String()
		if _, err := db.Exec(`INSERT INTO subscriptions
			(id, customer_id, vehicle_id, site_id, plan_name, price_cents, starts_at, expires_at, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active', ?)`,
			ids.StringToBin16(subID), custBin, vehBin, siteBin, "Mensual Carro", 50000, now, now.AddDate(0, 1, 0), now); err != nil {
			log.Printf("Error al crear abono: %v\n", err)
		}
		// Registrar el pago correspondiente al abono
		_, _ = db.Exec(`INSERT INTO payments
			(id, subscription_id, amount_cents, currency, method, status, created_at, updated_at)
			VALUES (?, ?, 50000, 'COP', 'card', 'completed', ?, ?)`,
			ids.StringToBin16(ids.UUIDv7String()), ids.StringToBin16(subID), now, now)
	}

	// 10. Dispositivo simulado (API key desde variable de entorno)
	deviceAPIKey := cfg.SeedDeviceAPIKey
	devID := queryID(db, `SELECT id FROM devices WHERE site_id = ? AND name = ? LIMIT 1`, siteBin, "Barrera Principal")
	if devID == "" {
		devID = ids.UUIDv7String()
		if _, err := db.Exec(`INSERT INTO devices (id, site_id, name, type, api_key_hash, is_active, created_at)
			VALUES (?, ?, ?, ?, ?, TRUE, ?)`,
			ids.StringToBin16(devID), siteBin, "Barrera Principal", "barrier", security.HashToken(deviceAPIKey), now); err != nil {
			log.Printf("Error al crear dispositivo: %v\n", err)
		}
	} else {
		// Asegura que la API key demo siga siendo válida.
		if _, err := db.Exec(`UPDATE devices SET api_key_hash = ? WHERE id = ?`, security.HashToken(deviceAPIKey), ids.StringToBin16(devID)); err != nil {
			log.Printf("Aviso al actualizar dispositivo: %v\n", err)
		}
	}

	log.Println("✅ Seed completado")
	log.Printf("📍 Sede: %s (ID: %s)", cfg.SeedSiteName, siteID)
	log.Println("🅿️ Espacios: A-01 a A-10")
	log.Printf("🚗 Vehículo abonado: %s", cfg.SeedVehiclePlate)
	log.Println("🚧 Dispositivo: Barrera Principal")
	log.Printf("   IDs: admin=%s operador=%s auditor=%s", adminID, operID, auditorID)
	log.Println("   Usuarios: admin, operador, auditor (contraseñas definidas por variables de entorno)")
}

func insertIgnoreAndGet(db *sql.DB, insert string, args []any, selectQuery string, selectArgs ...any) string {
	if _, err := db.Exec(insert, args...); err != nil {
		log.Printf("Aviso al insertar: %v\n", err)
	}
	return queryID(db, selectQuery, selectArgs...)
}

func queryID(db *sql.DB, query string, args ...any) string {
	var raw []byte
	if err := db.QueryRow(query, args...).Scan(&raw); err != nil {
		return ""
	}
	return ids.Bin16ToString(raw)
}
