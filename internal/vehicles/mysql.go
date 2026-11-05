package vehicles

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

func (r *mysqlRepo) Create(v *Vehicle) error {
	idBin := ids.UUIDv7()
	v.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO vehicles (id, plate_normalized, plate_raw, type_id, customer_id, is_active)
              VALUES (?, ?, ?, ?, ?, ?)`
	var customerID []byte
	if v.CustomerID != nil && *v.CustomerID != "" {
		customerID = ids.StringToBin16(*v.CustomerID)
	}
	_, err := r.db.Exec(query, idBin, v.PlateNormalized, v.PlateRaw, ids.StringToBin16(v.TypeID), customerID, v.IsActive)
	if err != nil {
		return apperr.From(err)
	}
	v.CreatedAt = time.Now().UTC()
	return nil
}

func (r *mysqlRepo) ByID(id string) (*Vehicle, error) {
	query := `SELECT id, plate_normalized, plate_raw, type_id, customer_id, is_active, created_at
              FROM vehicles WHERE id = ? LIMIT 1`
	return scanVehicle(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *mysqlRepo) ByPlate(plateNormalized string) (*Vehicle, error) {
	query := `SELECT id, plate_normalized, plate_raw, type_id, customer_id, is_active, created_at
              FROM vehicles WHERE plate_normalized = ? LIMIT 1`
	return scanVehicle(r.db.QueryRow(query, plateNormalized))
}

func (r *mysqlRepo) ListByCustomer(customerID string) ([]*Vehicle, error) {
	query := `SELECT id, plate_normalized, plate_raw, type_id, customer_id, is_active, created_at
              FROM vehicles WHERE customer_id = ? ORDER BY plate_normalized`
	rows, err := r.db.Query(query, ids.StringToBin16(customerID))
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()
	var vehicles []*Vehicle
	for rows.Next() {
		v, err := scanVehicle(rows)
		if err != nil {
			return nil, err
		}
		vehicles = append(vehicles, v)
	}
	return vehicles, rows.Err()
}

func (r *mysqlRepo) List(limit, offset int) ([]*Vehicle, error) {
	query := `SELECT id, plate_normalized, plate_raw, type_id, customer_id, is_active, created_at
	          FROM vehicles ORDER BY plate_normalized LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()

	var vehicles []*Vehicle
	for rows.Next() {
		v, err := scanVehicle(rows)
		if err != nil {
			return nil, err
		}
		vehicles = append(vehicles, v)
	}
	return vehicles, rows.Err()
}

func (r *mysqlRepo) Update(v *Vehicle) error {
	query := `UPDATE vehicles SET plate_normalized=?, plate_raw=?, type_id=?, customer_id=?, is_active=? WHERE id = ?`
	var customerID []byte
	if v.CustomerID != nil && *v.CustomerID != "" {
		customerID = ids.StringToBin16(*v.CustomerID)
	}
	_, err := r.db.Exec(query, v.PlateNormalized, v.PlateRaw, ids.StringToBin16(v.TypeID), customerID, v.IsActive, ids.StringToBin16(v.ID))
	return apperr.FromErr(err)
}

func scanVehicle(scanner interface {
	Scan(...any) error
}) (*Vehicle, error) {
	var v Vehicle
	var idBin, typeIDBin []byte
	var customerIDBin sql.NullString
	var createdAt time.Time
	err := scanner.Scan(&idBin, &v.PlateNormalized, &v.PlateRaw, &typeIDBin, &customerIDBin, &v.IsActive, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "vehículo no encontrado", 404, nil)
		}
		return nil, apperr.From(err)
	}
	v.ID = ids.Bin16ToString(idBin)
	v.TypeID = ids.Bin16ToString(typeIDBin)
	if customerIDBin.Valid {
		s := ids.Bin16ToString([]byte(customerIDBin.String))
		if s != "" {
			v.CustomerID = &s
		}
	}
	v.CreatedAt = createdAt
	return &v, nil
}
