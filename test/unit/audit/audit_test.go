package audit_test

import (
	"context"
	"testing"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/audit"
)

/* ---------- fakes ---------- */

type fakeRepo struct {
	entries []*audit.Entry
}

func (f *fakeRepo) Insert(e *audit.Entry) error {
	if e.ID == "" {
		e.ID = "entry-1"
	}
	f.entries = append(f.entries, e)
	return nil
}
func (f *fakeRepo) List(filter audit.Filter, limit, offset int) ([]*audit.Entry, error) {
	out := []*audit.Entry{}
	for _, e := range f.entries {
		if filter.EntityType != "" && e.EntityType != filter.EntityType {
			continue
		}
		if filter.Action != "" && e.Action != filter.Action {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
func (f *fakeRepo) Count(filter audit.Filter) (int, error) {
	list, err := f.List(filter, 0, 0)
	if err != nil {
		return 0, err
	}
	return len(list), nil
}

func newAuditService() (*audit.Service, *fakeRepo) {
	repo := &fakeRepo{}
	return audit.NewService(repo), repo
}

/* ---------- tests ---------- */

func TestRecord_OK(t *testing.T) {
	s, repo := newAuditService()
	ctx := context.Background()
	err := s.Record(ctx, "ticket.create", "ticket", "ticket-123", nil, map[string]any{"status": "open"})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(repo.entries) != 1 {
		t.Fatalf("se esperaba 1 entrada, obtuvo %d", len(repo.entries))
	}
	entry := repo.entries[0]
	if entry.Action != "ticket.create" || entry.EntityType != "ticket" || entry.EntityID == nil || *entry.EntityID != "ticket-123" {
		t.Fatalf("datos inesperados: %+v", entry)
	}
}

func TestRecord_MissingFields(t *testing.T) {
	s, _ := newAuditService()
	ctx := context.Background()
	if err := s.Record(ctx, "", "ticket", "ticket-1", nil, nil); err == nil {
		t.Fatal("acción vacía debe fallar")
	} else if !apperr.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("se esperaba INVALID_INPUT, obtuvo %v", err)
	}
	if err := s.Record(ctx, "ticket.create", "", "ticket-1", nil, nil); err == nil {
		t.Fatal("entityType vacío debe fallar")
	}
}

func TestRecord_WithUserContext(t *testing.T) {
	s, _ := newAuditService()
	ctx := context.Background()
	err := s.Record(ctx, "action", "entity", "id-1", nil, nil)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
}

func TestListAndCount(t *testing.T) {
	s, _ := newAuditService()
	ctx := context.Background()
	if err := s.Record(ctx, "ticket.create", "ticket", "t1", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(ctx, "ticket.settle", "ticket", "t2", nil, nil); err != nil {
		t.Fatal(err)
	}
	list, total, err := s.List(audit.Filter{}, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("se esperaban 2 entradas, total=%d len=%d", total, len(list))
	}
	list, total, err = s.List(audit.Filter{EntityType: "ticket"}, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 {
		t.Fatalf("se esperaban 2 entradas de tipo ticket, obtuvo %d", total)
	}
}
