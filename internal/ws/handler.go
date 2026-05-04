package ws

import (
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/IvanOplesnin/rc-car-server.git/internal/control"
)

type Handler struct {
	logger  *slog.Logger
	control *control.Service
	seq     atomic.Uint64
}

func NewHandler(logger *slog.Logger, controlService *control.Service) *Handler {
	return &Handler{
		logger:  logger,
		control: controlService,
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,

	// Для разработки разрешаем подключение с любого Origin.
	// Когда появится постоянный адрес Raspberry Pi внутри VPN, это можно ограничить.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("upgrade websocket", "error", err)
		return
	}
	defer conn.Close()

	h.logger.Info("websocket client connected", "remote_addr", r.RemoteAddr)

	h.sendState(conn, h.control.State())

	for {
		var msg IncomingMessage

		if err := conn.ReadJSON(&msg); err != nil {
			h.logger.Info("websocket client disconnected", "error", err)

			state := h.control.Stop()
			h.logger.Info(
				"motors stopped after websocket disconnect",
				"left", state.Left,
				"right", state.Right,
			)

			return
		}

		state := h.handleMessage(msg)

		h.sendState(conn, state)
	}
}

func (h *Handler) handleMessage(msg IncomingMessage) control.State {
	h.logger.Info(
		"received websocket message",
		"version", msg.Version,
		"type", msg.Type,
		"seq", msg.Seq,
	)

	if msg.Version != ProtocolVersion {
		return h.invalidState(ErrUnsupportedProtocolVersion)
	}

	switch msg.Type {
	case MessageTypeControl:
		return h.handleControlMessage(msg)

	case MessageTypeSystem:
		return h.handleSystemMessage(msg)

	default:
		return h.invalidState(ErrUnknownMessageType)
	}
}

func (h *Handler) handleControlMessage(msg IncomingMessage) control.State {
	if msg.Payload.Drive == nil {
		return h.invalidState(ErrMissingDrivePayload)
	}

	return h.control.Drive(control.DriveCommand{
		Left:  msg.Payload.Drive.Left,
		Right: msg.Payload.Drive.Right,
	})
}

func (h *Handler) handleSystemMessage(msg IncomingMessage) control.State {
	if msg.Payload.System == nil {
		return h.invalidState(ErrMissingSystemPayload)
	}

	switch msg.Payload.System.Command {
	case SystemCommandStop:
		return h.control.Stop()

	case SystemCommandEmergencyStop:
		// На этом этапе emergency_stop делает обычный stop.
		// Позже можно добавить отдельный метод control.EmergencyStop(reason).
		return h.control.Stop()

	default:
		return h.invalidState(ErrUnknownSystemCommand)
	}
}

func (h *Handler) invalidState(err error) control.State {
	state := h.control.State()
	state.LastCommandValid = false
	state.LastError = err.Error()

	h.logger.Warn("invalid websocket message", "error", err)

	return state
}

func (h *Handler) sendState(conn *websocket.Conn, state control.State) {
	msg := OutgoingMessage{
		Version:   ProtocolVersion,
		Type:      MessageTypeState,
		Seq:       h.seq.Add(1),
		Timestamp: time.Now().UnixMilli(),
		Payload: OutgoingPayload{
			Connection: ConnectionPayload{
				MotorConnected:  state.MotorConnected,
				CameraConnected: state.CameraConnected,
			},
			Drive: DrivePayload{
				Left:  state.Left,
				Right: state.Right,
			},
			Power: PowerPayload{
				BatteryVoltage: state.BatteryVoltage,
				BatteryPercent: state.BatteryPercent,
			},
			Network: NetworkPayload{
				RSSI: state.RSSI,
			},
			System: StateSystemPayload{
				UptimeMS: state.UptimeMS,
				FreeHeap: state.FreeHeap,
			},
			Safety: SafetyPayload{
				Failsafe:         state.Failsafe,
				LastCommandValid: state.LastCommandValid,
				LastError:        state.LastError,
			},
		},
	}

	if err := conn.WriteJSON(msg); err != nil {
		h.logger.Error("write websocket state", "error", err)
	}
}