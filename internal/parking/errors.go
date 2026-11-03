package parking

import "parking-backend/internal/platform/apperr"

var (
	ErrTicketNotFound  = apperr.New(apperr.ErrNotFound, "ticket no encontrado", 404, nil)
	ErrSpaceOccupied   = apperr.New(apperr.ErrSpaceOccupied, "el espacio ya está ocupado", 409, nil)
	ErrVehicleAlreadyIn = apperr.New(apperr.ErrVehicleAlreadyIn, "el vehículo ya está dentro", 409, nil)
	ErrTicketNotOpen   = apperr.New(apperr.ErrTicketNotOpen, "el ticket no está abierto", 400, nil)
	ErrTicketAlreadyPaid = apperr.New(apperr.ErrTicketAlreadyPaid, "el ticket ya fue pagado", 409, nil)
)
