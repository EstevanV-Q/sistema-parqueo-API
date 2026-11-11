package rateplans

import (
	"database/sql"
	"time"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/platform/outbox"
)

type mysqlRepo struct {
	db     *sql.DB
	outbox *outbox.Store
}

func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepo{db: db}
}

// NewSQLiteRepository usa el mismo contrato y esquema lógico que MySQL.
func NewSQLiteRepository(db *sql.DB) Repository {
	return &mysqlRepo{db: db, outbox: outbox.NewStore(db)}
}

func (r *mysqlRepo) Create(rp *RatePlan) error {
	idBin := ids.UUIDv7()
	rp.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO rate_plans (id, site_id, name, vehicle_type_id, currency, grace_minutes, minimum_cents, fraction_minutes, fraction_cents, daily_cap_cents, lost_ticket_cents, is_active, valid_from, valid_until, created_at)
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	var vtID []byte
	if rp.VehicleTypeID != nil && *rp.VehicleTypeID != "" {
		vtID = ids.StringToBin16(*rp.VehicleTypeID)
	}
	var validUntil any
	if rp.ValidUntil != nil {
		validUntil = *rp.ValidUntil
	}
	now := time.Now().UTC()
	tx, err := r.db.Begin()
	if err != nil {
		return apperr.From(err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(query, idBin, ids.StringToBin16(rp.SiteID), rp.Name, vtID, rp.Currency, rp.GraceMinutes, rp.MinimumCents, rp.FractionMinutes, rp.CentsPerFraction, rp.DailyCapCents, rp.LostTicketCents, rp.IsActive, rp.ValidFrom, validUntil, now)
	if err != nil {
		return apperr.From(err)
	}
	if err := r.outbox.EnqueueTx(tx, "rate_plan", rp.ID, "rate_plan.created", map[string]any{
		"rate_plan_id":      rp.ID,
		"site_id":           rp.SiteID,
		"name":              rp.Name,
		"vehicle_type_id":   rp.VehicleTypeID,
		"currency":          rp.Currency,
		"grace_minutes":     rp.GraceMinutes,
		"minimum_cents":     rp.MinimumCents,
		"fraction_minutes":  rp.FractionMinutes,
		"fraction_cents":    rp.CentsPerFraction,
		"daily_cap_cents":   rp.DailyCapCents,
		"lost_ticket_cents": rp.LostTicketCents,
		"is_active":         rp.IsActive,
		"valid_from":        rp.ValidFrom,
		"valid_until":       rp.ValidUntil,
	}); err != nil {
		return apperr.From(err)
	}
	if err := tx.Commit(); err != nil {
		return apperr.From(err)
	}
	rp.CreatedAt = now
	return nil
}

func (r *mysqlRepo) ByID(id string) (*RatePlan, error) {
	query := `SELECT id, site_id, name, vehicle_type_id, currency, grace_minutes, minimum_cents, fraction_minutes, fraction_cents, daily_cap_cents, lost_ticket_cents, is_active, valid_from, valid_until, created_at
              FROM rate_plans WHERE id = ? LIMIT 1`
	return scanRatePlan(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *mysqlRepo) BySite(siteID string) ([]*RatePlan, error) {
	query := `SELECT id, site_id, name, vehicle_type_id, currency, grace_minutes, minimum_cents, fraction_minutes, fraction_cents, daily_cap_cents, lost_ticket_cents, is_active, valid_from, valid_until, created_at
              FROM rate_plans WHERE site_id = ? ORDER BY name`
	rows, err := r.db.Query(query, ids.StringToBin16(siteID))
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()
	var plans []*RatePlan
	for rows.Next() {
		rp, err := scanRatePlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, rp)
	}
	return plans, rows.Err()
}

func (r *mysqlRepo) ActiveBySiteType(siteID, typeID string) (*RatePlan, error) {
	now := time.Now().UTC()
	query := `SELECT id, site_id, name, vehicle_type_id, currency, grace_minutes, minimum_cents, fraction_minutes, fraction_cents, daily_cap_cents, lost_ticket_cents, is_active, valid_from, valid_until, created_at
              FROM rate_plans
              WHERE site_id = ? AND is_active = TRUE
                AND (vehicle_type_id = ? OR vehicle_type_id IS NULL)
                AND valid_from <= ?
                AND (valid_until IS NULL OR valid_until >= ?)
              ORDER BY (vehicle_type_id IS NULL) ASC, valid_from DESC
              LIMIT 1`
	return scanRatePlan(r.db.QueryRow(query, ids.StringToBin16(siteID), ids.StringToBin16(typeID), now, now))
}

func (r *mysqlRepo) Update(rp *RatePlan) error {
	query := `UPDATE rate_plans SET name=?, currency=?, grace_minutes=?, minimum_cents=?, fraction_minutes=?, fraction_cents=?, daily_cap_cents=?, lost_ticket_cents=?, is_active=?, valid_from=?, valid_until=? WHERE id = ?`
	_, err := r.db.Exec(query, rp.Name, rp.Currency, rp.GraceMinutes, rp.MinimumCents, rp.FractionMinutes, rp.CentsPerFraction, rp.DailyCapCents, rp.LostTicketCents, rp.IsActive, rp.ValidFrom, rp.ValidUntil, ids.StringToBin16(rp.ID))
	return apperr.FromErr(err)
}

func scanRatePlan(scanner interface {
	Scan(...any) error
}) (*RatePlan, error) {
	var rp RatePlan
	var idBin, siteIDBin, vtID []byte
	var validUntil sql.NullTime
	var createdAt time.Time
	err := scanner.Scan(&idBin, &siteIDBin, &rp.Name, &vtID, &rp.Currency, &rp.GraceMinutes, &rp.MinimumCents, &rp.FractionMinutes, &rp.CentsPerFraction, &rp.DailyCapCents, &rp.LostTicketCents, &rp.IsActive, &rp.ValidFrom, &validUntil, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "tarifa no encontrada", 404, nil)
		}
		return nil, apperr.From(err)
	}
	rp.ID = ids.Bin16ToString(idBin)
	rp.SiteID = ids.Bin16ToString(siteIDBin)
	if len(vtID) > 0 {
		s := ids.Bin16ToString(vtID)
		if s != "" {
			rp.VehicleTypeID = &s
		}
	}
	if validUntil.Valid {
		rp.ValidUntil = &validUntil.Time
	}
	rp.CreatedAt = createdAt
	return &rp, nil
}
