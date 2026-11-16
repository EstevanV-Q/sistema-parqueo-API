package reporting

import (
	"database/sql"
	"time"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/shared/sitetime"
)

type sqlRepo struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &sqlRepo{db: db}
}

func (r *sqlRepo) Ocupacion(siteID string) (*Ocupacion, error) {
	var siteName string
	if err := r.db.QueryRow(`SELECT name FROM sites WHERE id = ? LIMIT 1`, ids.StringToBin16(siteID)).Scan(&siteName); err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "sede no encontrada", 404, nil)
		}
		return nil, apperr.From(err)
	}

	var total int
	var occupied sql.NullInt64
	row := r.db.QueryRow(`
		SELECT COUNT(*), SUM(CASE WHEN s.status = 'occupied' THEN 1 ELSE 0 END)
		FROM spaces s
		JOIN zones z ON s.zone_id = z.id
		WHERE z.site_id = ? AND s.is_active = TRUE`,
		ids.StringToBin16(siteID))
	if err := row.Scan(&total, &occupied); err != nil {
		return nil, apperr.From(err)
	}

	occ := int(occupied.Int64)
	free := total - occ
	pct := 0.0
	if total > 0 {
		pct = float64(occ) / float64(total) * 100
	}

	return &Ocupacion{
		SiteID:         siteID,
		SiteName:       siteName,
		TotalSpaces:    total,
		OccupiedSpaces: occ,
		FreeSpaces:     free,
		OccupancyPct:   pct,
		UpdatedAt:      time.Now().UTC(),
	}, nil
}

// Ingresos agrupa los pagos por día (fechas YYYY-MM-DD en hora de la sede).
func (r *sqlRepo) Ingresos(siteID string, from, to string) ([]IngresoDiario, error) {
	pays, err := r.payments(siteID, from, to)
	if err != nil {
		return nil, err
	}
	var out []IngresoDiario
	for _, p := range pays {
		if n := len(out); n > 0 && out[n-1].Date == p.date {
			out[n-1].Count++
			out[n-1].TotalCents += p.amount
			continue
		}
		out = append(out, IngresoDiario{Date: p.date, Count: 1, TotalCents: p.amount})
	}
	return out, nil
}

// CajaDiaria resume los pagos de un día por método (hora de la sede).
func (r *sqlRepo) CajaDiaria(siteID string, date string) (*CajaResumen, error) {
	pays, err := r.payments(siteID, date, date)
	if err != nil {
		return nil, err
	}
	res := &CajaResumen{SiteID: siteID, Date: date, ByMethod: map[string]int64{}}
	for _, p := range pays {
		res.ByMethod[p.method] += p.amount
		res.TotalCents += p.amount
		res.Count++
	}
	return res, nil
}

type payment struct {
	date   string
	method string
	amount int64
}

// payments devuelve los pagos completados de la sede entre dos fechas locales
// (inclusive). El día se calcula en Go con la zona de la sede porque DATE()
// no es portable entre MySQL y SQLite.
func (r *sqlRepo) payments(siteID, from, to string) ([]payment, error) {
	loc := sitetime.Location(r.db, siteID)
	start, end, err := sitetime.DayRange(from, to, loc)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`
		SELECT p.method, p.amount_cents, p.created_at
		FROM payments p
		LEFT JOIN tickets t ON p.ticket_id = t.id
		LEFT JOIN subscriptions s ON p.subscription_id = s.id
		WHERE p.status = 'completed' AND (t.site_id = ? OR s.site_id = ?)
		  AND p.created_at >= ? AND p.created_at < ?
		ORDER BY p.created_at`,
		ids.StringToBin16(siteID), ids.StringToBin16(siteID), start, end)
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()

	var out []payment
	for rows.Next() {
		var p payment
		var at time.Time
		if err := rows.Scan(&p.method, &p.amount, &at); err != nil {
			return nil, apperr.From(err)
		}
		p.date = at.In(loc).Format("2006-01-02")
		out = append(out, p)
	}
	return out, rows.Err()
}
