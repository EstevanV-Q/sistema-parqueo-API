package billing

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

func (r *mysqlRepo) Create(p *Payment) error {
	idBin := ids.UUIDv7()
	p.ID = ids.Bin16ToString(idBin)

	query := `INSERT INTO payments (id, ticket_id, subscription_id, amount_cents, currency, method, status, reference, cashier_user_id, idempotency_key)
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	var ticketID, subID, cashierID []byte
	if p.TicketID != nil && *p.TicketID != "" {
		ticketID = ids.StringToBin16(*p.TicketID)
	}
	if p.SubscriptionID != nil && *p.SubscriptionID != "" {
		subID = ids.StringToBin16(*p.SubscriptionID)
	}
	if p.CashierUserID != nil && *p.CashierUserID != "" {
		cashierID = ids.StringToBin16(*p.CashierUserID)
	}
	var idemKey []byte
	if p.IdempotencyKey != nil && *p.IdempotencyKey != "" {
		idemKey = []byte(*p.IdempotencyKey)
	}
	_, err := r.db.Exec(query, idBin, ticketID, subID, p.AmountCents, p.Currency, p.Method, p.Status, p.Reference, cashierID, idemKey)
	if err != nil {
		return apperr.From(err)
	}
	return nil
}

func (r *mysqlRepo) ByID(id string) (*Payment, error) {
	query := `SELECT id, ticket_id, subscription_id, amount_cents, currency, method, status, reference, cashier_user_id, idempotency_key, created_at, updated_at
              FROM payments WHERE id = ? LIMIT 1`
	return scanPayment(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *mysqlRepo) ByTicket(ticketID string) (*Payment, error) {
	query := `SELECT id, ticket_id, subscription_id, amount_cents, currency, method, status, reference, cashier_user_id, idempotency_key, created_at, updated_at
              FROM payments WHERE ticket_id = ? LIMIT 1`
	return scanPayment(r.db.QueryRow(query, ids.StringToBin16(ticketID)))
}

func (r *mysqlRepo) ByIdempotencyKey(key string) (*Payment, error) {
	query := `SELECT id, ticket_id, subscription_id, amount_cents, currency, method, status, reference, cashier_user_id, idempotency_key, created_at, updated_at
              FROM payments WHERE idempotency_key = ? LIMIT 1`
	return scanPayment(r.db.QueryRow(query, key))
}

func (r *mysqlRepo) UpdateStatus(id, status string) error {
	_, err := r.db.Exec(`UPDATE payments SET status=?, updated_at=? WHERE id = ?`, status, time.Now().UTC(), ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func (r *mysqlRepo) CajaDiaria(siteID string, date string) (*CajaDiaria, error) {
	// Los pagos se resuelven por ticket o por suscripción, ambos con site_id.
	query := `
		SELECT p.method, COUNT(*), COALESCE(SUM(p.amount_cents), 0)
		FROM payments p
		LEFT JOIN tickets t ON p.ticket_id = t.id
		LEFT JOIN subscriptions s ON p.subscription_id = s.id
		WHERE p.status = 'completed'
		  AND DATE(p.created_at) = ?
		  AND (t.site_id = ? OR s.site_id = ?)
		GROUP BY p.method
	`
	rows, err := r.db.Query(query, date, ids.StringToBin16(siteID), ids.StringToBin16(siteID))
	if err != nil {
		return nil, apperr.From(err)
	}
	defer rows.Close()

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

	return &CajaDiaria{
		SiteID:     siteID,
		Date:       date,
		TotalCents: total,
		Count:      count,
		ByMethod:   byMethod,
	}, nil
}

func (r *mysqlRepo) ListBySite(siteID string, from, to string, limit, offset int) ([]*Payment, error) {
	siteBin := ids.StringToBin16(siteID)
	query := `
		SELECT p.id, p.ticket_id, p.subscription_id, p.amount_cents, p.currency, p.method, p.status,
		       p.reference, p.cashier_user_id, p.idempotency_key, p.created_at, p.updated_at
		FROM payments p
		LEFT JOIN tickets t ON p.ticket_id = t.id
		LEFT JOIN subscriptions s ON p.subscription_id = s.id
		WHERE (t.site_id = ? OR s.site_id = ?)`
	args := []any{siteBin, siteBin}

	if from != "" {
		query += " AND DATE(p.created_at) >= ?"
		args = append(args, from)
	}
	if to != "" {
		query += " AND DATE(p.created_at) <= ?"
		args = append(args, to)
	}
	query += " ORDER BY p.created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, apperr.From(err)
	}
	defer rows.Close()
	var payments []*Payment
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

func scanPayment(scanner interface {
	Scan(...any) error
}) (*Payment, error) {
	var p Payment
	var idBin, ticketIDBin, subIDBin, cashierIDBin []byte
	var reference, idemKey sql.NullString
	var createdAt, updatedAt time.Time

	err := scanner.Scan(&idBin, &ticketIDBin, &subIDBin, &p.AmountCents, &p.Currency, &p.Method, &p.Status, &reference, &cashierIDBin, &idemKey, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "pago no encontrado", 404, nil)
		}
		return nil, apperr.From(err)
	}

	p.ID = ids.Bin16ToString(idBin)
	if ticketIDBin != nil {
		s := ids.Bin16ToString(ticketIDBin)
		p.TicketID = &s
	}
	if subIDBin != nil {
		s := ids.Bin16ToString(subIDBin)
		p.SubscriptionID = &s
	}
	if reference.Valid {
		p.Reference = &reference.String
	}
	if cashierIDBin != nil {
		s := ids.Bin16ToString(cashierIDBin)
		p.CashierUserID = &s
	}
	if idemKey.Valid {
		s := idemKey.String
		p.IdempotencyKey = &s
	}
	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt
	return &p, nil
}
