package devices

// Device representa un dispositivo de hardware (barrera, LPR, RFID, impresora).
type Device struct {
	ID            string  `json:"id"`
	SiteID        string  `json:"site_id"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	APIKeyHash    string  `json:"-"`
	IsActive      bool    `json:"is_active"`
	LastHeartbeat *string `json:"last_heartbeat,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
}

// DeviceEvent representa un evento recibido de un dispositivo.
type DeviceEvent struct {
	ID              string  `json:"id"`
	DeviceID        string  `json:"device_id"`
	EventID         string  `json:"event_id"`
	EventType       string  `json:"event_type"`
	PlateNormalized *string `json:"plate_normalized,omitempty"`
	Command         *string `json:"command,omitempty"`
	Decision        string  `json:"decision"`
	Reason          *string `json:"reason,omitempty"`
	CreatedAt       string  `json:"created_at,omitempty"`
}

// Tipos de dispositivo.
const (
	DeviceBarrier = "barrier"
	DeviceLPR     = "lpr"
	DeviceRFID    = "rfid"
	DevicePrinter = "printer"
)

// Tipos de evento.
const (
	EventEntryDetected = "entry_detected"
	EventExitDetected  = "exit_detected"
	EventHeartbeat     = "heartbeat"
	EventCommandAck    = "command_ack"
)

// Decisiones.
const (
	DecisionAllow = "allow"
	DecisionDeny  = "deny"
)
