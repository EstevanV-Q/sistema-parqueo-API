package parking

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

func (r *mysqlRepo) Create(t *Ticket) error {
	idBin := ids.UUIDv7()
	t.ID = ids.Bin16ToString(idBin)

	query := `INSERT INTO tickets (id, site_id, zone_id, space_id, vehicle_id, subscription_id, plate_normalized, plate_raw, type_id, rate_plan_id, entry_at, status, total_cents, paid_cents, is_lost)
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	var zoneID, spaceID, vehicleID, subID, ratePlanID []byte
	if t.ZoneID != nil && *t.ZoneID != "" {
		zoneID = ids.StringToBin16(*t.ZoneID)
	}
	if t.SpaceID != nil && *t.SpaceID != "" {
		spaceID = ids.StringToBin16(*t.SpaceID)
	}
	if t.VehicleID != nil && *t.VehicleID != "" {
		vehicleID = ids.StringToBin16(*t.VehicleID)
	}
	if t.SubscriptionID != nil && *t.SubscriptionID != "" {
		subID = ids.StringToBin16(*t.SubscriptionID)
	}
	if t.RatePlanID != nil && *t.RatePlanID != "" {
		ratePlanID = ids.StringToBin16(*t.RatePlanID)
	}
	_, err := r.db.Exec(query, idBin, ids.StringToBin16(t.SiteID), zoneID, spaceID, vehicleID, subID, t.PlateNormalized, t.PlateRaw, ids.StringToBin16(t.TypeID), ratePlanID, t.EntryAt, t.Status, t.TotalCents, t.PaidCents, t.IsLost)
	if err != nil {
		return apperr.From(err)
	}
	return nil
}

func (r *mysqlRepo) ByID(id string) (*Ticket, error) {
	query := `SELECT id, site_id, zone_id, space_id, vehicle_id, subscription_id, plate_normalized, plate_raw, type_id, rate_plan_id, entry_at, exit_at, status, total_cents, paid_cents, is_lost, created_at, updated_at
              FROM tickets WHERE id = ? LIMIT 1`
	return scanTicket(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *mysqlRepo) OpenBySpace(spaceID string) (*Ticket, error) {
	query := `SELECT id, site_id, zone_id, space_id, vehicle_id, subscription_id, plate_normalized, plate_raw, type_id, rate_plan_id, entry_at, exit_at, status, total_cents, paid_cents, is_lost, created_at, updated_at
              FROM tickets WHERE space_id = ? AND status = 'open' LIMIT 1`
	return scanTicket(r.db.QueryRow(query, ids.StringToBin16(spaceID)))
}

func (r *mysqlRepo) OpenByPlateSite(plateNormalized, siteID string) (*Ticket, error) {
	query := `SELECT id, site_id, zone_id, space_id, vehicle_id, subscription_id, plate_normalized, plate_raw, type_id, rate_plan_id, entry_at, exit_at, status, total_cents, paid_cents, is_lost, created_at, updated_at
              FROM tickets WHERE plate_normalized = ? AND site_id = ? AND status = 'open' LIMIT 1`
	return scanTicket(r.db.QueryRow(query, plateNormalized, ids.StringToBin16(siteID)))
}

func (r *mysqlRepo) LatestByPlateSite(plateNormalized, siteID string) (*Ticket, error) {
	query := `SELECT id, site_id, zone_id, space_id, vehicle_id, subscription_id, plate_normalized, plate_raw, type_id, rate_plan_id, entry_at, exit_at, status, total_cents, paid_cents, is_lost, created_at, updated_at
              FROM tickets WHERE plate_normalized = ? AND site_id = ? ORDER BY entry_at DESC LIMIT 1`
	return scanTicket(r.db.QueryRow(query, plateNormalized, ids.StringToBin16(siteID)))
}

func (r *mysqlRepo) SettleTicketTx(ticketID string, exitAt time.Time, totalCents, paidCents int64, method string, cashierUserID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return apperr.From(err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentStatus string
	ticketBin := ids.StringToBin16(ticketID)
	err = tx.QueryRow(`SELECT status FROM tickets WHERE id = ? FOR UPDATE`, ticketBin).Scan(&currentStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return apperr.From(err)
	}
	if currentStatus != TicketOpen {
		return ErrTicketNotOpen
	}

	now := time.Now().UTC()
	_, err = tx.Exec(`UPDATE tickets SET exit_at=?, status=?, total_cents=?, paid_cents=?, updated_at=? WHERE id = ?`,
		exitAt, TicketPaid, totalCents, paidCents, now, ticketBin)
	if err != nil {
		return apperr.From(err)
	}

	paymentID := ids.UUIDv7()
	var cashierBin []byte
	if cashierUserID != "" {
		cashierBin = ids.StringToBin16(cashierUserID)
	}
	if method == "" {
		method = "cash"
	}
	_, err = tx.Exec(`INSERT INTO payments (id, ticket_id, amount_cents, currency, method, status, cashier_user_id, created_at, updated_at)
		VALUES (?, ?, ?, 'COP', ?, 'completed', ?, ?, ?)`,
		paymentID, ticketBin, paidCents, method, cashierBin, now, now)
	if err != nil {
		return apperr.From(err)
	}

	_, err = tx.Exec(`INSERT INTO ticket_events (id, ticket_id, event_type, actor_user_id, details, created_at)
		VALUES (?, ?, 'exit', ?, 'vehiculo_pagado_salido', ?)`,
		ids.UUIDv7(), ticketBin, cashierBin, now)
	if err != nil {
		return apperr.From(err)
	}

	return tx.Commit()
}

func (r *mysqlRepo) Update(t *Ticket) error {
	query := `UPDATE tickets SET status=?, exit_at=?, total_cents=?, paid_cents=?, is_lost=?, updated_at=? WHERE id = ?`
	_, err := r.db.Exec(query, t.Status, t.ExitAt, t.TotalCents, t.PaidCents, t.IsLost, time.Now().UTC(), ids.StringToBin16(t.ID))
	return apperr.FromErr(err)
}

func (r *mysqlRepo) Close(id string, exitAt time.Time, totalCents, paidCents int64, status string) error {
	_, err := r.db.Exec(`UPDATE tickets SET exit_at=?, status=?, total_cents=?, paid_cents=?, updated_at=? WHERE id = ?`,
		exitAt, status, totalCents, paidCents, time.Now().UTC(), ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func (r *mysqlRepo) Events(ticketID string) ([]TicketEvent, error) {
	query := `SELECT id, ticket_id, event_type, actor_user_id, details, created_at
              FROM ticket_events WHERE ticket_id = ? ORDER BY created_at`
	rows, err := r.db.Query(query, ids.StringToBin16(ticketID))
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()
	var events []TicketEvent
	for rows.Next() {
		var e TicketEvent
		var idBin []byte
		var ticketIDBin []byte
		var actorID sql.NullString
		if err := rows.Scan(&idBin, &ticketIDBin, &e.EventType, &actorID, &e.Details, &e.CreatedAt); err != nil {
			return nil, apperr.From(err)
		}
		e.ID = ids.Bin16ToString(idBin)
		e.TicketID = ids.Bin16ToString(ticketIDBin)
		if actorID.Valid {
			e.ActorUserID = &actorID.String
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *mysqlRepo) CreateEvent(e *TicketEvent) error {
	_, err := r.db.Exec(`INSERT INTO ticket_events (id, ticket_id, event_type, actor_user_id, details, created_at)
              VALUES (?, ?, ?, ?, ?, ?)`,
		ids.UUIDv7(), ids.StringToBin16(e.TicketID), e.EventType, e.ActorUserID, e.Details, time.Now().UTC())
	return apperr.FromErr(err)
}

func (r *mysqlRepo) ListBySite(siteID string, status string, limit, offset int) ([]*Ticket, error) {
	query := `SELECT id, site_id, zone_id, space_id, vehicle_id, subscription_id, plate_normalized, plate_raw, type_id, rate_plan_id, entry_at, exit_at, status, total_cents, paid_cents, is_lost, created_at, updated_at
              FROM tickets WHERE site_id = ? AND (? = '' OR status = ?)
              ORDER BY entry_at DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, ids.StringToBin16(siteID), status, status, limit, offset)
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()
	var tickets []*Ticket
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

func (r *mysqlRepo) CountBySite(siteID string, status string) (int, error) {
	query := `SELECT COUNT(*) FROM tickets WHERE site_id = ? AND (? = '' OR status = ?)`
	var count int
	if err := r.db.QueryRow(query, ids.StringToBin16(siteID), status, status).Scan(&count); err != nil {
		return 0, apperr.From(err)
	}
	return count, nil
}

func scanTicket(scanner interface {
	Scan(...any) error
}) (*Ticket, error) {
	var t Ticket
	var idBin, siteIDBin, zoneIDBin, spaceIDBin, vehicleIDBin, subIDBin, typeIDBin, ratePlanIDBin []byte
	var exitAt sql.NullTime
	var status string
	var createdAt, updatedAt time.Time

	err := scanner.Scan(&idBin, &siteIDBin, &zoneIDBin, &spaceIDBin, &vehicleIDBin, &subIDBin, &t.PlateNormalized, &t.PlateRaw, &typeIDBin, &ratePlanIDBin, &t.EntryAt, &exitAt, &status, &t.TotalCents, &t.PaidCents, &t.IsLost, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, apperr.From(err)
	}

	t.ID = ids.Bin16ToString(idBin)
	t.SiteID = ids.Bin16ToString(siteIDBin)
	t.Status = status
	if zoneIDBin != nil {
		s := ids.Bin16ToString(zoneIDBin)
		t.ZoneID = &s
	}
	if spaceIDBin != nil {
		s := ids.Bin16ToString(spaceIDBin)
		t.SpaceID = &s
	}
	if vehicleIDBin != nil {
		s := ids.Bin16ToString(vehicleIDBin)
		t.VehicleID = &s
	}
	if subIDBin != nil {
		s := ids.Bin16ToString(subIDBin)
		t.SubscriptionID = &s
	}
	t.TypeID = ids.Bin16ToString(typeIDBin)
	if ratePlanIDBin != nil {
		s := ids.Bin16ToString(ratePlanIDBin)
		t.RatePlanID = &s
	}
	if exitAt.Valid {
		t.ExitAt = &exitAt.Time
	}
	t.CreatedAt = createdAt
	t.UpdatedAt = updatedAt
	return &t, nil
}

// ErrNotFound para tickets
var ErrNotFound = apperr.New(apperr.ErrNotFound, "ticket no encontrado", 404, nil)
