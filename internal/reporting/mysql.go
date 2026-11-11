package reporting

import (
	"database/sql"
	"time"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
)

type mysqlRepo struct {
	db *sql.DB
}

func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepo{db: db}
}

// NewSQLiteRepository usa el mismo contrato y esquema lógico que MySQL.
func NewSQLiteRepository(db *sql.DB) Repository {
	return &mysqlRepo{db: db}
}

func (r *mysqlRepo) Ocupacion(siteID string) (*Ocupacion, error) {
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

func (r *mysqlRepo) Ingresos(siteID string, from, to string) ([]IngresoDiario, error) {
	query := `
		SELECT DATE(p.created_at) AS fecha, COUNT(*), COALESCE(SUM(p.amount_cents), 0)
		FROM payments p
		LEFT JOIN tickets t ON p.ticket_id = t.id
		LEFT JOIN subscriptions s ON p.subscription_id = s.id
		WHERE p.status = 'completed' AND (t.site_id = ? OR s.site_id = ?)`
	args := []any{ids.StringToBin16(siteID), ids.StringToBin16(siteID)}

	if from != "" {
		query += " AND DATE(p.created_at) >= ?"
		args = append(args, from)
	}
	if to != "" {
		query += " AND DATE(p.created_at) <= ?"
		args = append(args, to)
	}
	query += " GROUP BY DATE(p.created_at) ORDER BY fecha"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()

	var ingresos []IngresoDiario
	for rows.Next() {
		var i IngresoDiario
		var fecha time.Time
		if err := rows.Scan(&fecha, &i.Count, &i.TotalCents); err != nil {
			return nil, apperr.From(err)
		}
		i.Date = fecha.Format("2006-01-02")
		ingresos = append(ingresos, i)
	}
	return ingresos, rows.Err()
}

func (r *mysqlRepo) CajaDiaria(siteID string, date string) (*CajaResumen, error) {
	query := `
		SELECT p.method, COUNT(*), COALESCE(SUM(p.amount_cents), 0)
		FROM payments p
		LEFT JOIN tickets t ON p.ticket_id = t.id
		LEFT JOIN subscriptions s ON p.subscription_id = s.id
		WHERE p.status = 'completed' AND DATE(p.created_at) = ?
		  AND (t.site_id = ? OR s.site_id = ?)
		GROUP BY p.method`
	rows, err := r.db.Query(query, date, ids.StringToBin16(siteID), ids.StringToBin16(siteID))
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()

	byMethod := map[string]int64{}
	var total int64
	var count int
	for rows.Next() {
		var method string
		var c int
		var amount int64
		if err := rows.Scan(&method, &c, &amount); err != nil {
			return nil, apperr.From(err)
		}
		byMethod[method] = amount
		total += amount
		count += c
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.From(err)
	}

	return &CajaResumen{
		SiteID:     siteID,
		Date:       date,
		TotalCents: total,
		Count:      count,
		ByMethod:   byMethod,
	}, nil
}
