package control

type MotorTelemetry struct {
	Left           int
	Right          int
	Failsafe       bool
	BatteryVoltage float64
	BatteryPercent int
	RSSI           int
	UptimeMS        uint64
	FreeHeap        uint64
}