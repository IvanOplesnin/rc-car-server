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

	BatteryVoltage  float64
	BatteryPercent  int
	RSSI            int
	UptimeMS        uint64
	FreeHeap        uint64
	LastTelemetryAt time.Time
}