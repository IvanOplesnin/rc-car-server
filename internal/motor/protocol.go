package motor

const (
	ProtocolVersion = 1

	MessageTypeControl = "control"
	MessageTypeSystem  = "system"
)

const (
	SystemCommandEmergencyStop = "emergency_stop"
)

type Message struct {
	Version   int     `json:"version"`
	Type      string  `json:"type"`
	Seq       uint64  `json:"seq"`
	Timestamp int64   `json:"timestamp"`
	Payload   Payload `json:"payload"`
}

type Payload struct {
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