package pricing

import (
	"testing"
	"time"

	"parking-backend/internal/shared/money"
	"parking-backend/internal/shared/plate"
)

func TestCalculate_DentroDeGracia(t *testing.T) {
	tests := []struct {
		name      string
		entry     time.Time
		exit      time.Time
		plan      Plan
		wantAmt   int64
		wantMin   int
	}{
		{
			name: "menos de gracia",
			entry: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			exit:  time.Date(2026, 1, 1, 10, 5, 0, 0, time.UTC),
			plan: Plan{GraceMinutes: 10, MinimumCents: 1000, FractionMinutes: 60, CentsPerFraction: 1500},
			wantAmt: 0,
			wantMin: 0,
		},
		{
			name: "exactamente gracia",
			entry: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			exit:  time.Date(2026, 1, 1, 10, 10, 0, 0, time.UTC),
			plan: Plan{GraceMinutes: 10, MinimumCents: 1000, FractionMinutes: 60, CentsPerFraction: 1500},
			wantAmt: 0,
			wantMin: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Calculate(Input{EntryAt: tt.entry, ExitAt: tt.exit, Plan: tt.plan})
			if q.Amount != money.Amount(tt.wantAmt) {
				t.Errorf("Calculate() monto = %v, quiere %v", q.Amount, tt.wantAmt)
			}
			if q.BillableMinutes != tt.wantMin {
				t.Errorf("Calculate() minutos = %v, quiere %v", q.BillableMinutes, tt.wantMin)
			}
		})
	}
}

func TestCalculate_CargoMinimo(t *testing.T) {
	entry := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	exit := time.Date(2026, 1, 1, 10, 20, 0, 0, time.UTC) // 20 min, 10 de gracia = 10 billable
	plan := Plan{GraceMinutes: 10, MinimumCents: 1000, FractionMinutes: 60, CentsPerFraction: 1500}

	q := Calculate(Input{EntryAt: entry, ExitAt: exit, Plan: plan})

	// 10 min billable = 1 fracción (60 min) = 1500, que ya supera el mínimo de 1000
	if q.Amount != money.Amount(1500) {
		t.Errorf("monto = %v, quiere 1500 (1 fracción)", q.Amount)
	}
	if q.BillableMinutes != 10 {
		t.Errorf("minutos = %v, quiere 10", q.BillableMinutes)
	}
}

func TestCalculate_FraccionCompleta(t *testing.T) {
	entry := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	exit := time.Date(2026, 1, 1, 11, 30, 0, 0, time.UTC) // 90 min, 10 gracia = 80 billable
	plan := Plan{GraceMinutes: 10, MinimumCents: 1000, FractionMinutes: 60, CentsPerFraction: 1500}

	q := Calculate(Input{EntryAt: entry, ExitAt: exit, Plan: plan})

	// 80 min = 1 fracción completa (60) + 20 restantes = 2 fracciones = 3000
	if q.Amount != money.Amount(3000) {
		t.Errorf("monto = %v, quiere 3000", q.Amount)
	}
	if q.BillableMinutes != 80 {
		t.Errorf("minutos = %v, quiere 80", q.BillableMinutes)
	}
}

func TestCalculate_TopeDiario(t *testing.T) {
	entry := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	exit := time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC) // 24 horas
	plan := Plan{GraceMinutes: 10, MinimumCents: 1000, FractionMinutes: 60, CentsPerFraction: 1500, DailyCapCents: 20000}

	q := Calculate(Input{EntryAt: entry, ExitAt: exit, Plan: plan})

	// 1439 billable min ≈ 24 fracciones = 36000, tope diario es 20000
	if q.Amount != money.Amount(20000) {
		t.Errorf("monto = %v, quiere 20000 (tope diario)", q.Amount)
	}
}

func TestCalculate_TicketPerdido(t *testing.T) {
	entry := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	exit := time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)
	plan := Plan{LostTicketCents: 25000}

	q := Calculate(Input{EntryAt: entry, ExitAt: exit, Plan: plan, LostTicket: true})

	if q.Amount != money.Amount(25000) {
		t.Errorf("monto = %v, quiere 25000 (ticket perdido)", q.Amount)
	}
	if q.BillableMinutes != 0 {
		t.Errorf("minutos = %v, quiere 0", q.BillableMinutes)
	}
}

func TestCalculate_MultiDias(t *testing.T) {
	entry := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	exit := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC) // 48 horas = 2 días
	plan := Plan{GraceMinutes: 10, MinimumCents: 1000, FractionMinutes: 60, CentsPerFraction: 1500, DailyCapCents: 20000}

	q := Calculate(Input{EntryAt: entry, ExitAt: exit, Plan: plan})

	// 2 días * 20000 = 40000 tope
	if q.Amount != money.Amount(40000) {
		t.Errorf("monto = %v, quiere 40000 (tope 2 días)", q.Amount)
	}
}

func TestMoney_String(t *testing.T) {
	if money.Amount(1250).String() != "12.50" {
		t.Errorf("String() = %s, quiere 12.50", money.Amount(1250).String())
	}
	if money.Amount(5).String() != "0.05" {
		t.Errorf("String() = %s, quiere 0.05", money.Amount(5).String())
	}
	if money.Amount(0).String() != "0.00" {
		t.Errorf("String() = %s, quiere 0.00", money.Amount(0).String())
	}
}

func TestPlate_Normalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ABC123", "ABC123"},
		{"abc123", "ABC123"},
		{"ABC-123", "ABC123"},
		{"ABC 123", "ABC123"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := plate.Normalize(tt.input); got != tt.want {
			t.Errorf("Normalize(%q) = %q, quiere %q", tt.input, got, tt.want)
		}
	}
}
