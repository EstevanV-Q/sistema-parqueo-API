package customers

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

func (r *mysqlRepo) Create(c *Customer) error {
	idBin := ids.UUIDv7()
	c.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO customers (id, document, name, phone, email, is_active)
              VALUES (?, ?, ?, ?, ?, ?)`
	var phone, email []byte
	if c.Phone != nil && *c.Phone != "" {
		phone = []byte(*c.Phone)
	}
	if c.Email != nil && *c.Email != "" {
		email = []byte(*c.Email)
	}
	_, err := r.db.Exec(query, idBin, c.Document, c.Name, phone, email, c.IsActive)
	if err != nil {
		return apperr.From(err)
	}
	c.CreatedAt = time.Now().UTC()
	return nil
}

func (r *mysqlRepo) ByID(id string) (*Customer, error) {
	query := `SELECT id, document, name, phone, email, is_active, created_at
              FROM customers WHERE id = ? LIMIT 1`
	return scanCustomer(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *mysqlRepo) ByDocument(document string) (*Customer, error) {
	query := `SELECT id, document, name, phone, email, is_active, created_at
              FROM customers WHERE document = ? LIMIT 1`
	return scanCustomer(r.db.QueryRow(query, document))
}

func (r *mysqlRepo) List(limit, offset int) ([]*Customer, error) {
	query := `SELECT id, document, name, phone, email, is_active, created_at
              FROM customers ORDER BY name LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, apperr.From(err)
	}
	defer rows.Close()
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
