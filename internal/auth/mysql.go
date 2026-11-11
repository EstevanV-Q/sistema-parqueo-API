package auth

import (
	"database/sql"
	"time"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/platform/outbox"
)

type mysqlRepo struct {
	db        *sql.DB
	outbox    *outbox.Store
	forUpdate bool
}

func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepo{db: db, forUpdate: true}
}

// NewSQLiteRepository usa el mismo contrato y esquema lógico que MySQL.
func NewSQLiteRepository(db *sql.DB) Repository {
	return &mysqlRepo{db: db, outbox: outbox.NewStore(db)}
}

func (r *mysqlRepo) lockClause() string {
	if r.forUpdate {
		return " FOR UPDATE"
	}
	return ""
}

func (r *mysqlRepo) Create(u *User) error {
	idBin := ids.UUIDv7()
	u.ID = ids.Bin16ToString(idBin)

	query := `INSERT INTO users (id, username, email, password_hash, role, site_id, active, created_at)
              VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	var siteID []byte
	if u.SiteID != nil {
		siteID = ids.StringToBin16(*u.SiteID)
	}
	now := time.Now().UTC()
	tx, err := r.db.Begin()
	if err != nil {
		return apperr.From(err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(query, idBin, u.Username, u.Email, u.PasswordHash, u.Role, siteID, u.Active, now)
	if err != nil {
		return apperr.From(err)
	}
	if err := r.outbox.EnqueueTx(tx, "user", u.ID, "user.created", map[string]any{
		"user_id":       u.ID,
		"username":      u.Username,
		"email":         u.Email,
		"password_hash": u.PasswordHash,
		"role":          u.Role,
		"site_id":       u.SiteID,
		"active":        u.Active,
	}); err != nil {
		return apperr.From(err)
	}
	if err := tx.Commit(); err != nil {
		return apperr.From(err)
	}
	u.CreatedAt = now.Format(time.RFC3339)
	return nil
}

func (r *mysqlRepo) ByUsername(username string) (*User, error) {
	query := `SELECT id, username, email, password_hash, role, site_id, active, created_at, last_login
              FROM users WHERE username = ? LIMIT 1`
	row := r.db.QueryRow(query, username)
	return scanUser(row)
}

func (r *mysqlRepo) ByEmail(email string) (*User, error) {
	query := `SELECT id, username, email, password_hash, role, site_id, active, created_at, last_login
              FROM users WHERE LOWER(email) = LOWER(?) LIMIT 1`
	row := r.db.QueryRow(query, email)
	return scanUser(row)
}

func (r *mysqlRepo) ByID(id string) (*User, error) {
	query := `SELECT id, username, email, password_hash, role, site_id, active, created_at, last_login
              FROM users WHERE id = ? LIMIT 1`
	row := r.db.QueryRow(query, ids.StringToBin16(id))
	return scanUser(row)
}

func (r *mysqlRepo) UpdateLastLogin(id string, at time.Time) error {
	_, err := r.db.Exec(`UPDATE users SET last_login = ? WHERE id = ?`, at, ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func (r *mysqlRepo) UpdatePasswordHash(id, passwordHash string) error {
	_, err := r.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func (r *mysqlRepo) CreateRefreshToken(t *RefreshToken) error {
	if t.ID == "" {
		t.ID = ids.UUIDv7String()
	}
	familyID := t.ID
	if t.FamilyID != nil && *t.FamilyID != "" {
		familyID = *t.FamilyID
	}
	t.FamilyID = &familyID
	query := `INSERT INTO refresh_tokens (id, user_id, token_hash, family_id, expires_at, created_at)
	          VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query, ids.StringToBin16(t.ID), ids.StringToBin16(t.UserID), t.TokenHash, ids.StringToBin16(familyID), t.ExpiresAt, time.Now().UTC())
	if err != nil {
		return apperr.From(err)
	}
	return nil
}

func (r *mysqlRepo) RefreshTokenByHash(hash string) (*RefreshToken, error) {
	query := `SELECT id, user_id, token_hash, family_id, replaced_by, expires_at, revoked_at, created_at
	          FROM refresh_tokens WHERE token_hash = ? LIMIT 1`
	return scanRefreshToken(r.db.QueryRow(query, hash))
}

// RotateRefreshToken valida y rota el token dentro de una transacción. Si el
// token ya fue revocado, devuelve ErrTokenReuse sin crear uno nuevo.
func (r *mysqlRepo) RotateRefreshToken(oldID string, newToken *RefreshToken) error {
	tx, err := r.db.Begin()
	if err != nil {
		return apperr.From(err)
	}
	defer func() { _ = tx.Rollback() }()

	var revokedAt sql.NullTime
	var familyID []byte
	err = tx.QueryRow(`SELECT revoked_at, family_id FROM refresh_tokens WHERE id = ?`+r.lockClause(),
		ids.StringToBin16(oldID)).Scan(&revokedAt, &familyID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return apperr.From(err)
	}
	if revokedAt.Valid {
		return ErrTokenReuse
	}

	family := ids.Bin16ToString(familyID)
	if family == "" {
		family = oldID
	}
	newToken.FamilyID = &family
	if newToken.ID == "" {
		newToken.ID = ids.UUIDv7String()
	}
	now := time.Now().UTC()
	_, err = tx.Exec(`INSERT INTO refresh_tokens (id, user_id, token_hash, family_id, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		ids.StringToBin16(newToken.ID), ids.StringToBin16(newToken.UserID), newToken.TokenHash,
		ids.StringToBin16(family), newToken.ExpiresAt, now)
	if err != nil {
		return apperr.From(err)
	}
	res, err := tx.Exec(`UPDATE refresh_tokens SET revoked_at = ?, replaced_by = ? WHERE id = ? AND revoked_at IS NULL`,
		now, ids.StringToBin16(newToken.ID), ids.StringToBin16(oldID))
	if err != nil {
		return apperr.From(err)
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		return ErrTokenReuse
	}
	if err := tx.Commit(); err != nil {
		return apperr.From(err)
	}
	return nil
}

func (r *mysqlRepo) RevokeRefreshToken(id string) error {
	_, err := r.db.Exec(`UPDATE refresh_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`,
		time.Now().UTC(), ids.StringToBin16(id))
	return apperr.FromErr(err)
}

func (r *mysqlRepo) RevokeRefreshTokenFamily(familyID string) error {
	_, err := r.db.Exec(`UPDATE refresh_tokens SET revoked_at = ?
		WHERE (family_id = ? OR id = ?) AND revoked_at IS NULL`,
		time.Now().UTC(), ids.StringToBin16(familyID), ids.StringToBin16(familyID))
	return apperr.FromErr(err)
}

func (r *mysqlRepo) RevokeUserRefreshTokens(userID string) error {
	_, err := r.db.Exec(`UPDATE refresh_tokens SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`,
		time.Now().UTC(), ids.StringToBin16(userID))
	return apperr.FromErr(err)
}

func scanRefreshToken(row *sql.Row) (*RefreshToken, error) {
	var t RefreshToken
	var idBin, userIDBin, familyIDBin, replacedByBin []byte
	var revokedAt sql.NullTime
	err := row.Scan(&idBin, &userIDBin, &t.TokenHash, &familyIDBin, &replacedByBin, &t.ExpiresAt, &revokedAt, &t.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, apperr.From(err)
	}
	t.ID = ids.Bin16ToString(idBin)
	t.UserID = ids.Bin16ToString(userIDBin)
	if len(familyIDBin) > 0 {
		s := ids.Bin16ToString(familyIDBin)
		t.FamilyID = &s
	}
	if len(replacedByBin) > 0 {
		s := ids.Bin16ToString(replacedByBin)
		t.ReplacedBy = &s
	}
	if revokedAt.Valid {
		t.RevokedAt = &revokedAt.Time
	}
	return &t, nil
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var idBin []byte
	var siteIDRaw interface{}
	var createdAt sql.NullString
	var lastLogin sql.NullString
	err := row.Scan(&idBin, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &siteIDRaw, &u.Active, &createdAt, &lastLogin)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, apperr.From(err)
	}

	u.ID = ids.Bin16ToString(idBin)
	if siteIDRaw != nil {
		if b, ok := siteIDRaw.([]byte); ok {
			s := ids.Bin16ToString(b)
			if s != "" {
				u.SiteID = &s
			}
		}
	}
	if createdAt.Valid {
		u.CreatedAt = createdAt.String
	}
	if lastLogin.Valid {
		u.LastLogin = &lastLogin.String
	}
	return &u, nil
}
