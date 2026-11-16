// Package sitetime convierte fechas locales de una sede en rangos UTC.
package sitetime

import (
	"database/sql"
	"time"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
)

// Location devuelve la zona horaria de la sede (UTC si no se puede cargar).
func Location(db *sql.DB, siteID string) *time.Location {
	var tz string
	if err := db.QueryRow(`SELECT timezone FROM sites WHERE id = ?`, ids.StringToBin16(siteID)).Scan(&tz); err == nil {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return time.UTC
}

// DayRange convierte fechas AAAA-MM-DD (inclusive) en [inicio, fin) UTC.
// Una fecha vacía deja ese extremo abierto.
func DayRange(from, to string, loc *time.Location) (start, end time.Time, err error) {
	start = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	end = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	if from != "" {
		d, err := time.ParseInLocation("2006-01-02", from, loc)
		if err != nil {
			return start, end, invalid()
		}
		start = d.UTC()
	}
	if to != "" {
		d, err := time.ParseInLocation("2006-01-02", to, loc)
		if err != nil {
			return start, end, invalid()
		}
		end = d.AddDate(0, 0, 1).UTC()
	}
	return start, end, nil
}

func invalid() error {
	return apperr.New(apperr.ErrInvalidInput, "fecha inválida (use AAAA-MM-DD)", 400, nil)
}
