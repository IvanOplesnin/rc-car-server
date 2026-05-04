package ws

type IncomingMessage struct {
	Type  string `json:"type"`
	Left  int    `json:"left,omitempty"`
	Right int    `json:"right,omitempty"`
}

type StateMessage struct {
	Type             string  `json:"type"`
	MotorConnected   bool    `json:"motor_connected"`
	CameraConnected  bool    `json:"camera_connected"`
	Left             int     `json:"left"`
	Right            int     `json:"right"`
	Failsafe         bool    `json:"failsafe"`
	LastCommandValid bool    `json:"last_command_valid"`
	Error            string  `json:"error,omitempty"`

	BatteryVoltage float64 `json:"battery_voltage"`
	RSSI           int     `json:"rssi"`
}