package ws

const (
	ProtocolVersion = 1

	MessageTypeControl = "control"
	MessageTypeSystem  = "system"
	MessageTypeState   = "state"
	MessageTypeError   = "error"
)

const (
	SystemCommandStop          = "stop"
	SystemCommandEmergencyStop = "emergency_stop"
)

type IncomingMessage struct {
	Version   int             `json:"version"`
	Type      string          `json:"type"`
	Seq       uint64          `json:"seq"`
	Timestamp int64           `json:"timestamp"`
	Payload   IncomingPayload `json:"payload"`
}

type IncomingPayload struct {
	Drive  *DrivePayload  `json:"drive,omitempty"`
	System *SystemPayload `json:"system,omitempty"`
}

type DrivePayload struct {
	Left  int `json:"left"`
	Right int `json:"right"`
}

type SystemPayload struct {
	Command string `json:"command"`
	Reason  string `json:"reason,omitempty"`
}

type OutgoingMessage struct {
	Version   int             `json:"version"`
	Type      string          `json:"type"`
	Seq       uint64          `json:"seq"`
	Timestamp int64           `json:"timestamp"`
	Payload   OutgoingPayload `json:"payload"`
}

type OutgoingPayload struct {
	Connection ConnectionPayload `json:"connection"`
	Drive      DrivePayload      `json:"drive"`
	Power      PowerPayload      `json:"power"`
	Network    NetworkPayload    `json:"network"`
	System     StateSystemPayload `json:"system"`
	Safety     SafetyPayload      `json:"safety"`
}

type ConnectionPayload struct {
	MotorConnected  bool `json:"motor_connected"`
	CameraConnected bool `json:"camera_connected"`
}

type PowerPayload struct {
	BatteryVoltage float64 `json:"battery_voltage"`
	BatteryPercent int     `json:"battery_percent"`
}

type NetworkPayload struct {
	RSSI int `json:"rssi"`
}

type StateSystemPayload struct {
	UptimeMS uint64 `json:"uptime_ms"`
	FreeHeap uint64 `json:"free_heap"`
}

type SafetyPayload struct {
	Failsafe         bool   `json:"failsafe"`
	LastCommandValid bool   `json:"last_command_valid"`
	LastError        string `json:"last_error,omitempty"`
}