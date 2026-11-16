package customers

import (
	"database/sql"
	"time"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
)

type sqlRepo struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &sqlRepo{db: db}
}

func (r *sqlRepo) Create(c *Customer) error {
	idBin := ids.UUIDv7()
	c.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO customers (id, document, name, phone, email, is_active, created_at)
              VALUES (?, ?, ?, ?, ?, ?, ?)`
	var phone, email any
	if c.Phone != nil && *c.Phone != "" {
		phone = *c.Phone
	}
	if c.Email != nil && *c.Email != "" {
		email = *c.Email
	}
	now := time.Now().UTC()
	tx, err := r.db.Begin()
	if err != nil {
		return apperr.From(err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(query, idBin, c.Document, c.Name, phone, email, c.IsActive, now)
	if err != nil {
		return apperr.From(err)
	}
	if err := tx.Commit(); err != nil {
		return apperr.From(err)
	}
	c.CreatedAt = now
	return nil
}

func (r *sqlRepo) ByID(id string) (*Customer, error) {
	query := `SELECT id, document, name, phone, email, is_active, created_at
              FROM customers WHERE id = ? LIMIT 1`
	return scanCustomer(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *sqlRepo) ByDocument(document string) (*Customer, error) {
	query := `SELECT id, document, name, phone, email, is_active, created_at
              FROM customers WHERE document = ? LIMIT 1`
	return scanCustomer(r.db.QueryRow(query, document))
}

func (r *sqlRepo) List(limit, offset int) ([]*Customer, error) {
	query := `SELECT id, document, name, phone, email, is_active, created_at
              FROM customers ORDER BY name LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()
	var customers []*Customer
	for rows.Next() {
		c, err := scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		customers = append(customers, c)
	}
	return customers, rows.Err()
}

func scanCustomer(scanner interface {
	Scan(...any) error
}) (*Customer, error) {
	var c Customer
	var idBin []byte
	var phone, email sql.NullString
	var createdAt time.Time
	err := scanner.Scan(&idBin, &c.Document, &c.Name, &phone, &email, &c.IsActive, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "cliente no encontrado", 404, nil)
		}
		return nil, apperr.From(err)
	}
	c.ID = ids.Bin16ToString(idBin)
	if phone.Valid {
		c.Phone = &phone.String
	}
	if email.Valid {
		c.Email = &email.String
	}
	c.CreatedAt = createdAt
	return &c, nil
}
