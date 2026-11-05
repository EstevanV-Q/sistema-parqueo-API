package pricing

import (
	"time"

	"parking-backend/internal/shared/money"
)

// Plan define los parámetros de una tarifa.
type Plan struct {
	ID               string
	GraceMinutes     int
	MinimumCents     int64
	FractionMinutes  int
	CentsPerFraction int64
	DailyCapCents    int64
	LostTicketCents  int64
}

// Quote es el resultado de una cotización de salida.
type Quote struct {
	BillableMinutes int
	Amount          money.Amount
	Breakdown       []BreakdownLine
}

// BreakdownLine explica cómo se calculó el monto.
type BreakdownLine struct {
	Concept string
	Minutes int
	Amount  money.Amount
}

// Input contiene los datos para calcular una cotización.
type Input struct {
	EntryAt    time.Time
	ExitAt     time.Time
	Plan       Plan
	LostTicket bool
	SiteTZ     *time.Location
}

// Calculate calcula el monto a pagar según el plan tarifario.
// Es una función pura: no toca DB ni HTTP.
func Calculate(in Input) Quote {
	// Si ticket perdido, cobro fijo
	if in.LostTicket {
		return Quote{
			BillableMinutes: 0,
			Amount:          money.Amount(in.Plan.LostTicketCents),
			Breakdown: []BreakdownLine{
				{Concept: "Ticket perdido", Minutes: 0, Amount: money.Amount(in.Plan.LostTicketCents)},
			},
		}
	}

	// Calcular minutos transcurridos en la zona horaria de la sede
	entry := in.EntryAt
	if in.SiteTZ != nil {
		entry = in.EntryAt.In(in.SiteTZ)
	}
	exit := in.ExitAt
	if in.SiteTZ != nil {
		exit = in.ExitAt.In(in.SiteTZ)
	}

	duration := exit.Sub(entry)
	totalMinutes := int(duration.Minutes())
	if totalMinutes < 0 {
		totalMinutes = 0
	}

	// Periodo de gracia: no se cobra
	billableMinutes := totalMinutes - in.Plan.GraceMinutes
	if billableMinutes < 0 {
		billableMinutes = 0
	}

	var amount int64
	var breakdown []BreakdownLine

	// Si está dentro de gracia, monto cero
	if billableMinutes == 0 {
		return Quote{
			BillableMinutes: 0,
			Amount:          0,
			Breakdown: []BreakdownLine{
				{Concept: "Dentro de gracia", Minutes: billableMinutes, Amount: 0},
			},
		}
	}

	// Calcular fracciones
	fullFractions := billableMinutes / in.Plan.FractionMinutes
	remainingMinutes := billableMinutes % in.Plan.FractionMinutes

	// Si hay minutos restantes, se cobra una fracción adicional (hora iniciada)
	if remainingMinutes > 0 {
		fullFractions++
	}

	fractionAmount := int64(fullFractions) * in.Plan.CentsPerFraction
	amount = fractionAmount

	breakdown = append(breakdown, BreakdownLine{
		Concept: "Fracciones", Minutes: billableMinutes, Amount: money.Amount(fractionAmount),
	})

	// Aplicar cargo mínimo si el calculado es menor
	if amount < in.Plan.MinimumCents {
		amount = in.Plan.MinimumCents
		breakdown = append(breakdown, BreakdownLine{
			Concept: "Ajuste a cargo mínimo", Minutes: 0, Amount: money.Amount(amount - fractionAmount),
		})
	}

	// Aplicar tope diario (por cada 24h iniciadas)
	if in.Plan.DailyCapCents > 0 {
		days := int(duration.Hours() / 24)
		if duration.Hours()-float64(days*24) > 0 {
			days++
		}
		if days < 1 {
			days = 1
		}
		maxAmount := int64(days) * in.Plan.DailyCapCents
		if amount > maxAmount {
			amount = maxAmount
			breakdown = append(breakdown, BreakdownLine{
				Concept: "Tope diario aplicado", Minutes: 0, Amount: money.Amount(amount),
			})
		}
	}

	return Quote{
		BillableMinutes: billableMinutes,
		Amount:          money.Amount(amount),
		Breakdown:       breakdown,
	}
}
