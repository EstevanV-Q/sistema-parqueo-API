package devices

// Device representa un dispositivo de hardware (barrera, LPR, RFID, impresora).
type Device struct {
	ID            string
	SiteID        string
	Name          string
	Type          string
	APIKeyHash    string
	IsActive      bool
	LastHeartbeat *string
	CreatedAt     string
}

// DeviceEvent representa un evento recibido de un dispositivo.
type DeviceEvent struct {
	ID              string
	DeviceID        string
	EventID         string
	EventType       string
	PlateNormalized *string
	Command         *string
	Decision        string
	Reason          *string
	CreatedAt       string
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
