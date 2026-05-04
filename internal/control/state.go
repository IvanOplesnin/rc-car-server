package control

import "time"

type State struct {
	MotorConnected   bool
	CameraConnected  bool
	Left             int
	Right            int
	Failsafe         bool
	LastCommandValid bool
	LastError        string
	LastCommandAt    time.Time

	BatteryVoltage float64
	RSSI           int
	LastTelemetryAt time.Time
}