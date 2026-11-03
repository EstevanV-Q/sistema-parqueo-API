package devices

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

func (r *mysqlRepo) Create(d *Device) error {
	idBin := ids.UUIDv7()
	d.ID = ids.Bin16ToString(idBin)
	query := `INSERT INTO devices (id, site_id, name, type, api_key_hash, is_active, last_heartbeat)
              VALUES (?, ?, ?, ?, ?, ?, ?)`
	var hb interface{}
	if d.LastHeartbeat != nil {
		hb = *d.LastHeartbeat
	}
	_, err := r.db.Exec(query, idBin, ids.StringToBin16(d.SiteID), d.Name, d.Type, d.APIKeyHash, d.IsActive, hb)
	if err != nil {
		return apperr.From(err)
	}
	return nil
}

func (r *mysqlRepo) ByID(id string) (*Device, error) {
	query := `SELECT id, site_id, name, type, api_key_hash, is_active, last_heartbeat, created_at
              FROM devices WHERE id = ? LIMIT 1`
	return scanDevice(r.db.QueryRow(query, ids.StringToBin16(id)))
}

func (r *mysqlRepo) ByAPIKey(apiKeyHash string) (*Device, error) {
	query := `SELECT id, site_id, name, type, api_key_hash, is_active, last_heartbeat, created_at
              FROM devices WHERE api_key_hash = ? LIMIT 1`
	return scanDevice(r.db.QueryRow(query, apiKeyHash))
}

func (r *mysqlRepo) UpdateHeartbeat(id string, at time.Time) error {
	_, err := r.db.Exec(`UPDATE devices SET last_heartbeat=? WHERE id = ?`, at, ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func (r *mysqlRepo) CreateEvent(e *DeviceEvent) error {
	_, err := r.db.Exec(`INSERT INTO device_events (id, device_id, event_id, event_type, plate_normalized, command, decision, reason, created_at)
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ids.UUIDv7(), ids.StringToBin16(e.DeviceID), e.EventID, e.EventType, nullString(e.PlateNormalized), nullString(e.Command), e.Decision, nullString(e.Reason), time.Now().UTC())
	return apperr.FromErr(err)
}

func nullString(v *string) any {
	if v == nil || *v == "" {
		return nil
	}
	return *v
}

func (r *mysqlRepo) EventByID(eventID string) (*DeviceEvent, error) {
	query := `SELECT id, device_id, event_id, event_type, plate_normalized, command, decision, reason, created_at
              FROM device_events WHERE event_id = ? LIMIT 1`
	return scanEvent(r.db.QueryRow(query, eventID))
}

func (r *mysqlRepo) ListBySite(siteID string) ([]*Device, error) {
	query := `SELECT id, site_id, name, type, api_key_hash, is_active, last_heartbeat, created_at
	          FROM devices WHERE site_id = ? ORDER BY name`
	rows, err := r.db.Query(query, ids.StringToBin16(siteID))
	if err != nil {
		return nil, apperr.From(err)
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func scanDevice(scanner interface {
	Scan(...any) error
}) (*Device, error) {
	var d Device
	var idBin, siteIDBin []byte
	var hb sql.NullString
	var createdAt time.Time
	err := scanner.Scan(&idBin, &siteIDBin, &d.Name, &d.Type, &d.APIKeyHash, &d.IsActive, &hb, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "dispositivo no encontrado", 404, nil)
		}
		return nil, apperr.From(err)
	}
	d.ID = ids.Bin16ToString(idBin)
	d.SiteID = ids.Bin16ToString(siteIDBin)
	if hb.Valid {
		d.LastHeartbeat = &hb.String
	}
	d.CreatedAt = createdAt.Format(time.RFC3339)
	return &d, nil
}

func scanEvent(scanner interface {
	Scan(...any) error
}) (*DeviceEvent, error) {
	var e DeviceEvent
	var idBin, deviceIDBin []byte
	var plate sql.NullString
	var command, reason sql.NullString
	var createdAt time.Time
	err := scanner.Scan(&idBin, &deviceIDBin, &e.EventID, &e.EventType, &plate, &command, &e.Decision, &reason, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "evento de dispositivo no encontrado", 404, nil)
		}
		return nil, apperr.From(err)
	}
	e.ID = ids.Bin16ToString(idBin)
	e.DeviceID = ids.Bin16ToString(deviceIDBin)
	if plate.Valid {
		e.PlateNormalized = &plate.String
	}
	if command.Valid {
		e.Command = &command.String
	}
	if reason.Valid {
		e.Reason = &reason.String
	}
	e.CreatedAt = createdAt.Format(time.RFC3339)
	return &e, nil
}
