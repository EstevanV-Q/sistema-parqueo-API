// Comando seed carga datos de demostración de forma idempotente.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"parking-backend/internal/config"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/platform/security"
)

func main() {
	cfg := config.Load()
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

	now := time.Now().UTC()

	// 1. Sede
	siteID := insertIgnoreAndGet(db,
		`INSERT IGNORE INTO sites (id, name, code, timezone, currency, capacity_total, is_active, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, TRUE, ?)`,
		[]any{ids.StringToBin16(ids.UUIDv7String()), "Centro Comercial Plaza", "PLAZA-01", "America/Bogota", "COP", 50, now},
		`SELECT id FROM sites WHERE code = ?`, "PLAZA-01",
	)
	if siteID == "" {
		fmt.Println("No se pudo obtener la sede")
		os.Exit(1)
	}
	siteBin := ids.StringToBin16(siteID)

	// 2. Usuarios
	adminHash, _ := security.HashPassword("Admin123!")
	adminID := insertIgnoreAndGet(db,
		`INSERT IGNORE INTO users (id, username, email, password_hash, role, site_id, active, created_at)
		 VALUES (?, ?, ?, ?, ?, NULL, TRUE, ?)`,
		[]any{ids.StringToBin16(ids.UUIDv7String()), "admin", "admin@parqueo.local", adminHash, "admin", now},
		`SELECT id FROM users WHERE username = ?`, "admin",
	)

	operHash, _ := security.HashPassword("Oper123!")
	operID := insertIgnoreAndGet(db,
		`INSERT IGNORE INTO users (id, username, email, password_hash, role, site_id, active, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, TRUE, ?)`,
		[]any{ids.StringToBin16(ids.UUIDv7String()), "operador", "operador@parqueo.local", operHash, "operator", siteBin, now},
		`SELECT id FROM users WHERE username = ?`, "operador",
	)
	// Si el operador existía sin sede, se le asigna la sede demo.
	_, _ = db.Exec(`UPDATE users SET site_id = ? WHERE username = ?`, siteBin, "operador")

	auditorHash, _ := security.HashPassword("Audit123!")
	auditorID := insertIgnoreAndGet(db,
		`INSERT IGNORE INTO users (id, username, email, password_hash, role, site_id, active, created_at)
		 VALUES (?, ?, ?, ?, ?, NULL, TRUE, ?)`,
		[]any{ids.StringToBin16(ids.UUIDv7String()), "auditor", "auditor@parqueo.local", auditorHash, "auditor", now},
		`SELECT id FROM users WHERE username = ?`, "auditor",
	)

	// 3. Zona
	zoneID := queryID(db, `SELECT id FROM zones WHERE site_id = ? AND name = ? LIMIT 1`, siteBin, "General - Carros")
	if zoneID == "" {
		zoneID = ids.UUIDv7String()
		if _, err := db.Exec(`INSERT INTO zones (id, site_id, name, type, capacity, is_active, created_at)
			VALUES (?, ?, ?, ?, ?, TRUE, ?)`,
			ids.StringToBin16(zoneID), siteBin, "General - Carros", "car", 30, now); err != nil {
			fmt.Printf("Error al crear zona: %v\n", err)
		}
	}
	zoneBin := ids.StringToBin16(zoneID)

	// 4. Espacios (10 carros)
	for i := 1; i <= 10; i++ {
		code := fmt.Sprintf("A-%02d", i)
		_, err = db.Exec(`INSERT IGNORE INTO spaces (id, zone_id, code, type, status, is_active, created_at)
			VALUES (?, ?, ?, ?, 'free', TRUE, ?)`,
			ids.StringToBin16(ids.UUIDv7String()), zoneBin, code, "car", now)
		if err != nil {
			fmt.Printf("Error en espacio %s: %v\n", code, err)
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
			fmt.Printf("Error al crear tarifa: %v\n", err)
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
	plateNorm := "XYZ987"
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
			fmt.Printf("Error al crear abono: %v\n", err)
		}
		// Registrar el pago correspondiente al abono
		_, _ = db.Exec(`INSERT INTO payments
			(id, subscription_id, amount_cents, currency, method, status, created_at, updated_at)
			VALUES (?, ?, 50000, 'COP', 'card', 'completed', ?, ?)`,
			ids.StringToBin16(ids.UUIDv7String()), ids.StringToBin16(subID), now, now)
	}

	// 10. Dispositivo simulado
	deviceAPIKey := "device-api-key-001"
	devID := queryID(db, `SELECT id FROM devices WHERE site_id = ? AND name = ? LIMIT 1`, siteBin, "Barrera Principal")
	if devID == "" {
		devID = ids.UUIDv7String()
		if _, err := db.Exec(`INSERT INTO devices (id, site_id, name, type, api_key_hash, is_active, created_at)
			VALUES (?, ?, ?, ?, ?, TRUE, ?)`,
			ids.StringToBin16(devID), siteBin, "Barrera Principal", "barrier", security.HashToken(deviceAPIKey), now); err != nil {
			fmt.Printf("Error al crear dispositivo: %v\n", err)
		}
	} else {
		// Asegura que la API key demo siga siendo válida.
		_, _ = db.Exec(`UPDATE devices SET api_key_hash = ? WHERE id = ?`, security.HashToken(deviceAPIKey), ids.StringToBin16(devID))
	}

	fmt.Println("✅ Seed completado")
	fmt.Println("🔑 Admin:    admin / Admin123! (todas las sedes)")
	fmt.Println("🔑 Operador: operador / Oper123! (sede PLAZA-01)")
	fmt.Println("🔑 Auditor:  auditor / Audit123! (solo lectura)")
	fmt.Printf("📍 Sede: %s (ID: %s)\n", "Centro Comercial Plaza", siteID)
	fmt.Println("🅿️ Espacios: A-01 a A-10")
	fmt.Printf("🚗 Vehículo abonado: XYZ987\n")
	fmt.Printf("🚧 Dispositivo: Barrera Principal (X-Device-Key: %s)\n", deviceAPIKey)
	fmt.Printf("   IDs: admin=%s operador=%s auditor=%s\n", adminID, operID, auditorID)
}

func insertIgnoreAndGet(db *sql.DB, insert string, args []any, selectQuery string, selectArgs ...any) string {
	if _, err := db.Exec(insert, args...); err != nil {
		fmt.Printf("Aviso al insertar: %v\n", err)
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
