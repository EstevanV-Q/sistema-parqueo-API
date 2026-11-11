package audit

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/platform/outbox"
)

type mysqlRepo struct {
	db     *sql.DB
	outbox *outbox.Store
}

// NewMySQLRepository crea el repositorio de auditoría.
func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepo{db: db}
}

// NewSQLiteRepository usa el mismo contrato y esquema lógico que MySQL.
func NewSQLiteRepository(db *sql.DB) Repository {
	return &mysqlRepo{db: db, outbox: outbox.NewStore(db)}
}

func (r *mysqlRepo) Insert(e *Entry) error {
	idBin := ids.UUIDv7()
	if e.ID == "" {
		e.ID = ids.Bin16ToString(idBin)
	} else if b := ids.StringToBin16(e.ID); len(b) == 16 {
		idBin = b
	}

	var userID []byte
	if e.UserID != nil {
		userID = ids.StringToBin16(*e.UserID)
	}
	var siteID []byte
	if e.SiteID != nil {
		siteID = ids.StringToBin16(*e.SiteID)
	}
	var entityID []byte
	if e.EntityID != nil {
		entityID = ids.StringToBin16(*e.EntityID)
	}

	oldJSON, err := marshalNullable(e.OldValues)
	if err != nil {
		return apperr.From(err)
	}
	newJSON, err := marshalNullable(e.NewValues)
	if err != nil {
		return apperr.From(err)
	}

	now := time.Now().UTC()
	tx, err := r.db.Begin()
	if err != nil {
		return apperr.From(err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`
		INSERT INTO audit_log (id, user_id, site_id, action, entity_type, entity_id, old_values, new_values, ip_address, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		idBin, userID, siteID, e.Action, e.EntityType, entityID, oldJSON, newJSON, e.IPAddress, e.UserAgent, now)
	if err != nil {
		return apperr.From(err)
	}
	if err := r.outbox.EnqueueTx(tx, "audit", e.ID, "audit.created", map[string]any{
		"audit_id":    e.ID,
		"user_id":     e.UserID,
		"site_id":     e.SiteID,
		"action":      e.Action,
		"entity_type": e.EntityType,
		"entity_id":   e.EntityID,
		"old_values":  string(oldJSON),
		"new_values":  string(newJSON),
		"ip_address":  e.IPAddress,
		"user_agent":  e.UserAgent,
		"created_at":  now,
	}); err != nil {
		return apperr.From(err)
	}
	if err := tx.Commit(); err != nil {
		return apperr.From(err)
	}
	e.CreatedAt = now
	return nil
}

func marshalNullable(v map[string]any) ([]byte, error) {
	if len(v) == 0 {
		return nil, nil
	}
	return json.Marshal(v)
}

func (r *mysqlRepo) List(f Filter, limit, offset int) ([]*Entry, error) {
	where, args := buildWhere(f)
	query := `SELECT id, user_id, site_id, action, entity_type, entity_id, old_values, new_values, ip_address, user_agent, created_at
	          FROM audit_log ` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, apperr.From(err)
	}
	defer func() { _ = rows.Close() }()

	var entries []*Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *mysqlRepo) Count(f Filter) (int, error) {
	where, args := buildWhere(f)
	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM audit_log `+where, args...).Scan(&total); err != nil {
		return 0, apperr.From(err)
	}
	return total, nil
}

func buildWhere(f Filter) (string, []any) {
	var clauses []string
	var args []any

	if f.EntityType != "" {
		clauses = append(clauses, "entity_type = ?")
		args = append(args, f.EntityType)
	}
	if f.EntityID != "" {
		clauses = append(clauses, "entity_id = ?")
		args = append(args, ids.StringToBin16(f.EntityID))
	}
	if f.UserID != "" {
		clauses = append(clauses, "user_id = ?")
		args = append(args, ids.StringToBin16(f.UserID))
	}
	if f.SiteID != "" {
		clauses = append(clauses, "site_id = ?")
		args = append(args, ids.StringToBin16(f.SiteID))
	}
	if f.Action != "" {
		clauses = append(clauses, "action = ?")
		args = append(args, f.Action)
	}
	if f.From != "" {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		clauses = append(clauses, "created_at <= ?")
		args = append(args, f.To)
	}

	if len(clauses) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func scanEntry(scanner interface {
	Scan(...any) error
}) (*Entry, error) {
	var e Entry
	var idBin []byte
	var userIDBin, siteIDBin, entityIDBin []byte
	var oldRaw, newRaw []byte
	var ip, ua sql.NullString

	err := scanner.Scan(&idBin, &userIDBin, &siteIDBin, &e.Action, &e.EntityType, &entityIDBin, &oldRaw, &newRaw, &ip, &ua, &e.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "registro de auditoría no encontrado", 404, nil)
		}
		return nil, apperr.From(err)
	}

	e.ID = ids.Bin16ToString(idBin)
	if len(userIDBin) > 0 {
		s := ids.Bin16ToString(userIDBin)
		e.UserID = &s
	}
	if len(siteIDBin) > 0 {
		s := ids.Bin16ToString(siteIDBin)
		e.SiteID = &s
	}
	if len(entityIDBin) > 0 {
		s := ids.Bin16ToString(entityIDBin)
		e.EntityID = &s
	}
	if len(oldRaw) > 0 {
		_ = json.Unmarshal(oldRaw, &e.OldValues)
	}
	if len(newRaw) > 0 {
		_ = json.Unmarshal(newRaw, &e.NewValues)
	}
	if ip.Valid {
		e.IPAddress = &ip.String
	}
	if ua.Valid {
		e.UserAgent = &ua.String
	}
	return &e, nil
}
