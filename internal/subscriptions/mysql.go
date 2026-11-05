package subscriptions

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

func (r *mysqlRepo) Create(s *Subscription) error {
	idBin := ids.UUIDv7()
	s.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO subscriptions (id, customer_id, vehicle_id, site_id, plan_name, price_cents, starts_at, expires_at, status)
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query, idBin, ids.StringToBin16(s.CustomerID), ids.StringToBin16(s.VehicleID), ids.StringToBin16(s.SiteID), s.PlanName, s.PriceCents, s.StartsAt, s.ExpiresAt, s.Status)
	if err != nil {
		return apperr.From(err)
	}
	return nil
}

func (r *mysqlRepo) ByID(id string) (*Subscription, error) {
	query := `SELECT id, customer_id, vehicle_id, site_id, plan_name, price_cents, starts_at, expires_at, status, created_at
              FROM subscriptions WHERE id = ? LIMIT 1`
	return scanSub(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *mysqlRepo) ByVehicle(vehicleID string) (*Subscription, error) {
	query := `SELECT id, customer_id, vehicle_id, site_id, plan_name, price_cents, starts_at, expires_at, status, created_at
              FROM subscriptions WHERE vehicle_id = ? ORDER BY expires_at DESC LIMIT 1`
	return scanSub(r.db.QueryRow(query, ids.StringToBin16(vehicleID)))
}

func (r *mysqlRepo) ActiveByVehicleSite(vehicleID, siteID string, now time.Time) (*Subscription, error) {
	query := `SELECT id, customer_id, vehicle_id, site_id, plan_name, price_cents, starts_at, expires_at, status, created_at
              FROM subscriptions WHERE vehicle_id = ? AND site_id = ? AND status = 'active' AND starts_at <= ? AND expires_at >= ?
              ORDER BY expires_at DESC LIMIT 1`
	return scanSub(r.db.QueryRow(query, ids.StringToBin16(vehicleID), ids.StringToBin16(siteID), now, now))
}

func (r *mysqlRepo) ActiveByPlateSite(plateNormalized, siteID string, now time.Time) (*Subscription, error) {
	query := `SELECT s.id, s.customer_id, s.vehicle_id, s.site_id, s.plan_name, s.price_cents, s.starts_at, s.expires_at, s.status, s.created_at
              FROM subscriptions s
              JOIN vehicles v ON s.vehicle_id = v.id
              WHERE v.plate_normalized = ? AND s.site_id = ? AND s.status = 'active' AND s.starts_at <= ? AND s.expires_at >= ?
              ORDER BY s.expires_at DESC LIMIT 1`
	return scanSub(r.db.QueryRow(query, plateNormalized, ids.StringToBin16(siteID), now, now))
}

func (r *mysqlRepo) UpdateStatus(id, status string) error {
	_, err := r.db.Exec(`UPDATE subscriptions SET status=? WHERE id = ?`, status, ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func (r *mysqlRepo) ListBySite(siteID string, status string, limit, offset int) ([]*Subscription, error) {
	query := `SELECT id, customer_id, vehicle_id, site_id, plan_name, price_cents, starts_at, expires_at, status, created_at
              FROM subscriptions WHERE site_id = ? AND (? = '' OR status = ?)
              ORDER BY created_at DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, ids.StringToBin16(siteID), status, status, limit, offset)
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()
	var subs []*Subscription
	for rows.Next() {
		s, err := scanSub(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func scanSub(scanner interface {
	Scan(...any) error
}) (*Subscription, error) {
	var s Subscription
	var idBin, customerIDBin, vehicleIDBin, siteIDBin []byte
	var createdAt time.Time
	err := scanner.Scan(&idBin, &customerIDBin, &vehicleIDBin, &siteIDBin, &s.PlanName, &s.PriceCents, &s.StartsAt, &s.ExpiresAt, &s.Status, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "suscripción no encontrada", 404, nil)
		}
		return nil, apperr.From(err)
	}
	s.ID = ids.Bin16ToString(idBin)
	s.CustomerID = ids.Bin16ToString(customerIDBin)
	s.VehicleID = ids.Bin16ToString(vehicleIDBin)
	s.SiteID = ids.Bin16ToString(siteIDBin)
	s.CreatedAt = createdAt
	return &s, nil
}
