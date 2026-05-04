package control

type MotorTelemetry struct {
	BatteryVoltage float64
	RSSI           int
	Left           int
	Right          int
	Failsafe       bool
}