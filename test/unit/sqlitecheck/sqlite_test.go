package sqlitecheck

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSQLiteBLOBAndTimeRoundTrip(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(`CREATE TABLE t (id BLOB PRIMARY KEY, ts TIMESTAMP NOT NULL, nullable TIMESTAMP NULL)`); err != nil {
		t.Fatal(err)
	}

	id := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	now := time.Date(2026, 11, 9, 12, 30, 0, 0, time.UTC)
	if _, err := db.Exec(`INSERT INTO t (id, ts, nullable) VALUES (?, ?, ?)`, id, now, nil); err != nil {
		t.Fatal(err)
	}

	var gotID []byte
	var gotTime time.Time
	var nullable sql.NullTime
	if err := db.QueryRow(`SELECT id, ts, nullable FROM t`).Scan(&gotID, &gotTime, &nullable); err != nil {
		t.Fatal(err)
	}
	if len(gotID) != 16 {
		t.Fatalf("id len = %d", len(gotID))
	}
	if !gotTime.Equal(now) {
		t.Fatalf("time = %v, want %v", gotTime, now)
	}
	if nullable.Valid {
		t.Fatal("nullable should be null")
	}
}
