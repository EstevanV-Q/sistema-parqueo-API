package catalog

import (
	"database/sql"
	"time"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
)

type sqlSiteRepo struct {
	db *sql.DB
}

func NewSiteRepo(db *sql.DB) SiteRepository {
	return &sqlSiteRepo{db: db}
}

func (r *sqlSiteRepo) Create(s *Site) error {
	idBin := ids.UUIDv7()
	s.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO sites (id, name, code, timezone, currency, capacity_total, is_active)
              VALUES (?, ?, ?, ?, ?, ?, ?)`
	nowTime := time.Now().UTC()
	tx, err := r.db.Begin()
	if err != nil {
		return apperr.From(err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(query, idBin, s.Name, s.Code, s.Timezone, s.Currency, s.CapacityTotal, s.IsActive)
	if err != nil {
		return apperr.From(err)
	}
	if err := tx.Commit(); err != nil {
		return apperr.From(err)
	}
	now := nowTime.Format(time.RFC3339)
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (r *sqlSiteRepo) ByID(id string) (*Site, error) {
	query := `SELECT id, name, code, timezone, currency, capacity_total, is_active, created_at, updated_at
              FROM sites WHERE id = ? LIMIT 1`
	return scanSite(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *sqlSiteRepo) ByCode(code string) (*Site, error) {
	query := `SELECT id, name, code, timezone, currency, capacity_total, is_active, created_at, updated_at
              FROM sites WHERE code = ? LIMIT 1`
	return scanSite(r.db.QueryRow(query, code))
}

func (r *sqlSiteRepo) List() ([]*Site, error) {
	query := `SELECT id, name, code, timezone, currency, capacity_total, is_active, created_at, updated_at
              FROM sites ORDER BY name`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()
	var sites []*Site
	for rows.Next() {
		s, err := scanSite(rows)
		if err != nil {
			return nil, err
		}
		sites = append(sites, s)
	}
	return sites, rows.Err()
}

func (r *sqlSiteRepo) Update(s *Site) error {
	query := `UPDATE sites SET name=?, code=?, timezone=?, currency=?, capacity_total=?, is_active=?, updated_at=?
              WHERE id = ?`
	_, err := r.db.Exec(query, s.Name, s.Code, s.Timezone, s.Currency, s.CapacityTotal, s.IsActive, time.Now().UTC(), ids.StringToBin16(s.ID))
	return apperr.FromErr(err)
}

func (r *sqlSiteRepo) Delete(id string) error {
	_, err := r.db.Exec(`UPDATE sites SET is_active=FALSE, updated_at=? WHERE id = ?`, time.Now().UTC(), ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func scanSite(scanner interface {
	Scan(...any) error
}) (*Site, error) {
	var s Site
	var idBin []byte
	var createdAt, updatedAt sql.NullString
	err := scanner.Scan(&idBin, &s.Name, &s.Code, &s.Timezone, &s.Currency, &s.CapacityTotal, &s.IsActive, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "sitio no encontrado", 404, nil)
		}
		return nil, apperr.From(err)
	}
	s.ID = ids.Bin16ToString(idBin)
	if createdAt.Valid {
		s.CreatedAt = createdAt.String
	}
	if updatedAt.Valid {
		s.UpdatedAt = updatedAt.String
	}
	return &s, nil
}

type sqlZoneRepo struct {
	db *sql.DB
}

func NewZoneRepo(db *sql.DB) ZoneRepository {
	return &sqlZoneRepo{db: db}
}

func (r *sqlZoneRepo) Create(z *Zone) error {
	idBin := ids.UUIDv7()
	z.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO zones (id, site_id, name, type, capacity, is_active)
              VALUES (?, ?, ?, ?, ?, ?)`
	now := time.Now().UTC()
	tx, err := r.db.Begin()
	if err != nil {
		return apperr.From(err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(query, idBin, ids.StringToBin16(z.SiteID), z.Name, z.Type, z.Capacity, z.IsActive)
	if err != nil {
		return apperr.From(err)
	}
	if err := tx.Commit(); err != nil {
		return apperr.From(err)
	}
	z.CreatedAt = now.Format(time.RFC3339)
	return nil
}

func (r *sqlZoneRepo) ByID(id string) (*Zone, error) {
	query := `SELECT id, site_id, name, type, capacity, is_active, created_at
              FROM zones WHERE id = ? LIMIT 1`
	return scanZone(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *sqlZoneRepo) ListBySite(siteID string) ([]*Zone, error) {
	query := `SELECT id, site_id, name, type, capacity, is_active, created_at
              FROM zones WHERE site_id = ? ORDER BY name`
	rows, err := r.db.Query(query, ids.StringToBin16(siteID))
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()
	var zones []*Zone
	for rows.Next() {
		z, err := scanZone(rows)
		if err != nil {
			return nil, err
		}
		zones = append(zones, z)
	}
	return zones, rows.Err()
}

func (r *sqlZoneRepo) Update(z *Zone) error {
	query := `UPDATE zones SET name=?, type=?, capacity=?, is_active=? WHERE id = ?`
	_, err := r.db.Exec(query, z.Name, z.Type, z.Capacity, z.IsActive, ids.StringToBin16(z.ID))
	return apperr.FromErr(err)
}

func (r *sqlZoneRepo) Delete(id string) error {
	_, err := r.db.Exec(`UPDATE zones SET is_active=FALSE WHERE id = ?`, ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func scanZone(scanner interface {
	Scan(...any) error
}) (*Zone, error) {
	var z Zone
	var idBin []byte
	var siteIDBin []byte
	var createdAt sql.NullString
	err := scanner.Scan(&idBin, &siteIDBin, &z.Name, &z.Type, &z.Capacity, &z.IsActive, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "zona no encontrada", 404, nil)
		}
		return nil, apperr.From(err)
	}
	z.ID = ids.Bin16ToString(idBin)
	z.SiteID = ids.Bin16ToString(siteIDBin)
	if createdAt.Valid {
		z.CreatedAt = createdAt.String
	}
	return &z, nil
}

type sqlSpaceRepo struct {
	db *sql.DB
}

func NewSpaceRepo(db *sql.DB) SpaceRepository {
	return &sqlSpaceRepo{db: db}
}

func (r *sqlSpaceRepo) Create(s *Space) error {
	idBin := ids.UUIDv7()
	s.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO spaces (id, zone_id, code, type, status, is_active)
              VALUES (?, ?, ?, ?, ?, ?)`
	nowTime := time.Now().UTC()
	tx, err := r.db.Begin()
	if err != nil {
		return apperr.From(err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(query, idBin, ids.StringToBin16(s.ZoneID), s.Code, s.Type, s.Status, s.IsActive)
	if err != nil {
		return apperr.From(err)
	}
	if err := tx.Commit(); err != nil {
		return apperr.From(err)
	}
	now := nowTime.Format(time.RFC3339)
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (r *sqlSpaceRepo) ByID(id string) (*Space, error) {
	query := `SELECT id, zone_id, code, type, status, is_active, created_at, updated_at
              FROM spaces WHERE id = ? LIMIT 1`
	return scanSpace(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *sqlSpaceRepo) ByCode(zoneID, code string) (*Space, error) {
	query := `SELECT id, zone_id, code, type, status, is_active, created_at, updated_at
              FROM spaces WHERE zone_id = ? AND code = ? LIMIT 1`
	return scanSpace(r.db.QueryRow(query, ids.StringToBin16(zoneID), code))
}

func (r *sqlSpaceRepo) ListByZone(zoneID string) ([]*Space, error) {
	query := `SELECT id, zone_id, code, type, status, is_active, created_at, updated_at
              FROM spaces WHERE zone_id = ? ORDER BY code`
	rows, err := r.db.Query(query, ids.StringToBin16(zoneID))
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()
	var spaces []*Space
	for rows.Next() {
		s, err := scanSpace(rows)
		if err != nil {
			return nil, err
		}
		spaces = append(spaces, s)
	}
	return spaces, rows.Err()
}

func (r *sqlSpaceRepo) Update(s *Space) error {
	query := `UPDATE spaces SET code=?, type=?, status=?, is_active=?, updated_at=? WHERE id = ?`
	_, err := r.db.Exec(query, s.Code, s.Type, s.Status, s.IsActive, time.Now().UTC(), ids.StringToBin16(s.ID))
	return apperr.FromErr(err)
}

func (r *sqlSpaceRepo) UpdateStatus(id, status string) error {
	_, err := r.db.Exec(`UPDATE spaces SET status=?, updated_at=? WHERE id = ?`, status, time.Now().UTC(), ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func (r *sqlSpaceRepo) Delete(id string) error {
	_, err := r.db.Exec(`UPDATE spaces SET is_active=FALSE, updated_at=? WHERE id = ?`, time.Now().UTC(), ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func scanSpace(scanner interface {
	Scan(...any) error
}) (*Space, error) {
	var s Space
	var idBin []byte
	var zoneIDBin []byte
	var createdAt, updatedAt sql.NullString
	err := scanner.Scan(&idBin, &zoneIDBin, &s.Code, &s.Type, &s.Status, &s.IsActive, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "espacio no encontrado", 404, nil)
		}
		return nil, apperr.From(err)
	}
	s.ID = ids.Bin16ToString(idBin)
	s.ZoneID = ids.Bin16ToString(zoneIDBin)
	if createdAt.Valid {
		s.CreatedAt = createdAt.String
	}
	if updatedAt.Valid {
		s.UpdatedAt = updatedAt.String
	}
	return &s, nil
}

type sqlVehicleTypeRepo struct {
	db *sql.DB
}

func NewVehicleTypeRepo(db *sql.DB) VehicleTypeRepository {
	return &sqlVehicleTypeRepo{db: db}
}

func (r *sqlVehicleTypeRepo) Create(v *VehicleType) error {
	idBin := ids.UUIDv7()
	v.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO vehicle_types (id, name, code, is_active)
              VALUES (?, ?, ?, ?)`
	now := time.Now().UTC()
	tx, err := r.db.Begin()
	if err != nil {
		return apperr.From(err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(query, idBin, v.Name, v.Code, v.IsActive)
	if err != nil {
		return apperr.From(err)
	}
	if err := tx.Commit(); err != nil {
		return apperr.From(err)
	}
	v.CreatedAt = now.Format(time.RFC3339)
	return nil
}

func (r *sqlVehicleTypeRepo) ByID(id string) (*VehicleType, error) {
	query := `SELECT id, name, code, is_active, created_at
              FROM vehicle_types WHERE id = ? LIMIT 1`
	return scanVehicleType(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *sqlVehicleTypeRepo) ByCode(code string) (*VehicleType, error) {
	query := `SELECT id, name, code, is_active, created_at
              FROM vehicle_types WHERE code = ? LIMIT 1`
	return scanVehicleType(r.db.QueryRow(query, code))
}

func (r *sqlVehicleTypeRepo) Update(v *VehicleType) error {
	_, err := r.db.Exec(`UPDATE vehicle_types SET name=?, code=?, is_active=? WHERE id = ?`,
		v.Name, v.Code, v.IsActive, ids.StringToBin16(v.ID))
	return apperr.FromErr(err)
}

func (r *sqlVehicleTypeRepo) Delete(id string) error {
	_, err := r.db.Exec(`UPDATE vehicle_types SET is_active=FALSE WHERE id = ?`, ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func (r *sqlVehicleTypeRepo) List() ([]*VehicleType, error) {
	query := `SELECT id, name, code, is_active, created_at
              FROM vehicle_types ORDER BY name`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()
	var types []*VehicleType
	for rows.Next() {
		v, err := scanVehicleType(rows)
		if err != nil {
			return nil, err
		}
		types = append(types, v)
	}
	return types, rows.Err()
}

func scanVehicleType(scanner interface {
	Scan(...any) error
}) (*VehicleType, error) {
	var v VehicleType
	var idBin []byte
	var createdAt sql.NullString
	err := scanner.Scan(&idBin, &v.Name, &v.Code, &v.IsActive, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "tipo de vehículo no encontrado", 404, nil)
		}
		return nil, apperr.From(err)
	}
	v.ID = ids.Bin16ToString(idBin)
	if createdAt.Valid {
		v.CreatedAt = createdAt.String
	}
	return &v, nil
}
