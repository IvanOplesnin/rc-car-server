package httpserver

import (
	"encoding/json"
	"net/http"
	"time"
)

type healthResponse struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

type stateResponse struct {
	MotorConnected   bool   `json:"motor_connected"`
	CameraConnected  bool   `json:"camera_connected"`
	Left             int    `json:"left"`
	Right            int    `json:"right"`
	Failsafe         bool   `json:"failsafe"`
	LastCommandValid bool   `json:"last_command_valid"`
	LastError        string `json:"last_error"`
	LastCommandAt    string `json:"last_command_at,omitempty"`

	BatteryVoltage  float64 `json:"battery_voltage"`
	BatteryPercent  int     `json:"battery_percent"`
	RSSI            int     `json:"rssi"`
	UptimeMS        uint64  `json:"uptime_ms"`
	FreeHeap        uint64  `json:"free_heap"`
	LastTelemetryAt string  `json:"last_telemetry_at,omitempty"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status: "ok",
		Time:   time.Now().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, resp, s.logger)
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	state := s.stateProvider.State()

	resp := stateResponse{
		MotorConnected:   state.MotorConnected,
		CameraConnected:  state.CameraConnected,
		Left:             state.Left,
		Right:            state.Right,
		Failsafe:         state.Failsafe,
		LastCommandValid: state.LastCommandValid,
		LastError:        state.LastError,
		BatteryVoltage:   state.BatteryVoltage,
		BatteryPercent:   state.BatteryPercent,
		RSSI:             state.RSSI,
		UptimeMS:         state.UptimeMS,
		FreeHeap:         state.FreeHeap,
	}

	if !state.LastCommandAt.IsZero() {
		resp.LastCommandAt = state.LastCommandAt.Format(time.RFC3339)
	}

	if !state.LastTelemetryAt.IsZero() {
		resp.LastTelemetryAt = state.LastTelemetryAt.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, resp, s.logger)
}

func writeJSON(w http.ResponseWriter, statusCode int, data any, logger interface {
	Error(msg string, args ...any)
}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error("write json response", "error", err)
	}
}