package catalog

import (
	"database/sql"
	"time"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
)

type mysqlSiteRepo struct {
	db *sql.DB
}

func NewMySQLSiteRepo(db *sql.DB) SiteRepository {
	return &mysqlSiteRepo{db: db}
}

func (r *mysqlSiteRepo) Create(s *Site) error {
	idBin := ids.UUIDv7()
	s.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO sites (id, name, code, timezone, currency, capacity_total, is_active)
              VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query, idBin, s.Name, s.Code, s.Timezone, s.Currency, s.CapacityTotal, s.IsActive)
	if err != nil {
		return apperr.From(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (r *mysqlSiteRepo) ByID(id string) (*Site, error) {
	query := `SELECT id, name, code, timezone, currency, capacity_total, is_active, created_at, updated_at
              FROM sites WHERE id = ? LIMIT 1`
	return scanSite(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *mysqlSiteRepo) ByCode(code string) (*Site, error) {
	query := `SELECT id, name, code, timezone, currency, capacity_total, is_active, created_at, updated_at
              FROM sites WHERE code = ? LIMIT 1`
	return scanSite(r.db.QueryRow(query, code))
}

func (r *mysqlSiteRepo) List() ([]*Site, error) {
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

func (r *mysqlSiteRepo) Update(s *Site) error {
	query := `UPDATE sites SET name=?, code=?, timezone=?, currency=?, capacity_total=?, is_active=?, updated_at=?
              WHERE id = ?`
	_, err := r.db.Exec(query, s.Name, s.Code, s.Timezone, s.Currency, s.CapacityTotal, s.IsActive, time.Now().UTC(), ids.StringToBin16(s.ID))
	return apperr.FromErr(err)
}

func (r *mysqlSiteRepo) Delete(id string) error {
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

type mysqlZoneRepo struct {
	db *sql.DB
}

func NewMySQLZoneRepo(db *sql.DB) ZoneRepository {
	return &mysqlZoneRepo{db: db}
}

func (r *mysqlZoneRepo) Create(z *Zone) error {
	idBin := ids.UUIDv7()
	z.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO zones (id, site_id, name, type, capacity, is_active)
              VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query, idBin, ids.StringToBin16(z.SiteID), z.Name, z.Type, z.Capacity, z.IsActive)
	if err != nil {
		return apperr.From(err)
	}
	z.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

func (r *mysqlZoneRepo) ByID(id string) (*Zone, error) {
	query := `SELECT id, site_id, name, type, capacity, is_active, created_at
              FROM zones WHERE id = ? LIMIT 1`
	return scanZone(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *mysqlZoneRepo) ListBySite(siteID string) ([]*Zone, error) {
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

func (r *mysqlZoneRepo) Update(z *Zone) error {
	query := `UPDATE zones SET name=?, type=?, capacity=?, is_active=? WHERE id = ?`
	_, err := r.db.Exec(query, z.Name, z.Type, z.Capacity, z.IsActive, ids.StringToBin16(z.ID))
	return apperr.FromErr(err)
}

func (r *mysqlZoneRepo) Delete(id string) error {
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

type mysqlSpaceRepo struct {
	db *sql.DB
}

func NewMySQLSpaceRepo(db *sql.DB) SpaceRepository {
	return &mysqlSpaceRepo{db: db}
}

func (r *mysqlSpaceRepo) Create(s *Space) error {
	idBin := ids.UUIDv7()
	s.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO spaces (id, zone_id, code, type, status, is_active)
              VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query, idBin, ids.StringToBin16(s.ZoneID), s.Code, s.Type, s.Status, s.IsActive)
	if err != nil {
		return apperr.From(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (r *mysqlSpaceRepo) ByID(id string) (*Space, error) {
	query := `SELECT id, zone_id, code, type, status, is_active, created_at, updated_at
              FROM spaces WHERE id = ? LIMIT 1`
	return scanSpace(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *mysqlSpaceRepo) ByCode(zoneID, code string) (*Space, error) {
	query := `SELECT id, zone_id, code, type, status, is_active, created_at, updated_at
              FROM spaces WHERE zone_id = ? AND code = ? LIMIT 1`
	return scanSpace(r.db.QueryRow(query, ids.StringToBin16(zoneID), code))
}

func (r *mysqlSpaceRepo) ListByZone(zoneID string) ([]*Space, error) {
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

func (r *mysqlSpaceRepo) Update(s *Space) error {
	query := `UPDATE spaces SET code=?, type=?, status=?, is_active=?, updated_at=? WHERE id = ?`
	_, err := r.db.Exec(query, s.Code, s.Type, s.Status, s.IsActive, time.Now().UTC(), ids.StringToBin16(s.ID))
	return apperr.FromErr(err)
}

func (r *mysqlSpaceRepo) UpdateStatus(id, status string) error {
	_, err := r.db.Exec(`UPDATE spaces SET status=?, updated_at=? WHERE id = ?`, status, time.Now().UTC(), ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func (r *mysqlSpaceRepo) Delete(id string) error {
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

type mysqlVehicleTypeRepo struct {
	db *sql.DB
}

func NewMySQLVehicleTypeRepo(db *sql.DB) VehicleTypeRepository {
	return &mysqlVehicleTypeRepo{db: db}
}

func (r *mysqlVehicleTypeRepo) Create(v *VehicleType) error {
	idBin := ids.UUIDv7()
	v.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO vehicle_types (id, name, code, is_active)
              VALUES (?, ?, ?, ?)`
	_, err := r.db.Exec(query, idBin, v.Name, v.Code, v.IsActive)
	if err != nil {
		return apperr.From(err)
	}
	v.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

func (r *mysqlVehicleTypeRepo) ByID(id string) (*VehicleType, error) {
	query := `SELECT id, name, code, is_active, created_at
              FROM vehicle_types WHERE id = ? LIMIT 1`
	return scanVehicleType(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *mysqlVehicleTypeRepo) ByCode(code string) (*VehicleType, error) {
	query := `SELECT id, name, code, is_active, created_at
              FROM vehicle_types WHERE code = ? LIMIT 1`
	return scanVehicleType(r.db.QueryRow(query, code))
}

func (r *mysqlVehicleTypeRepo) Update(v *VehicleType) error {
	_, err := r.db.Exec(`UPDATE vehicle_types SET name=?, code=?, is_active=? WHERE id = ?`,
		v.Name, v.Code, v.IsActive, ids.StringToBin16(v.ID))
	return apperr.FromErr(err)
}

func (r *mysqlVehicleTypeRepo) Delete(id string) error {
	_, err := r.db.Exec(`UPDATE vehicle_types SET is_active=FALSE WHERE id = ?`, ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func (r *mysqlVehicleTypeRepo) List() ([]*VehicleType, error) {
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
