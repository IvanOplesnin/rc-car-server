package ws

import (
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/IvanOplesnin/rc-car-server.git/internal/access"
	"github.com/IvanOplesnin/rc-car-server.git/internal/control"
)

type Handler struct {
	logger        *slog.Logger
	control       *control.Service
	accessManager *access.Manager
	seq           atomic.Uint64
}

func NewHandler(
	logger *slog.Logger,
	controlService *control.Service,
	accessManager *access.Manager,
) *Handler {
	return &Handler{
		logger:        logger,
		control:       controlService,
		accessManager: accessManager,
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
			h.logger.Info(
				"websocket client disconnected",
				"remote_addr", r.RemoteAddr,
				"error", err,
			)
		
			decision := h.accessManager.IsOwner(r.RemoteAddr)
			if decision.Allowed {
				state := h.control.Stop()
		
				h.logger.Info(
					"motors stopped after owner websocket disconnect",
					"remote_addr", r.RemoteAddr,
					"client", decision.Client,
					"left", state.Left,
					"right", state.Right,
				)
			} else {
				h.logger.Info(
					"websocket disconnected without motor stop",
					"remote_addr", r.RemoteAddr,
					"reason", decision.Reason,
					"client", decision.Client,
					"owner", decision.Owner,
				)
			}
		
			return
		}

		state := h.handleMessage(r.RemoteAddr, msg)

		h.sendState(conn, state)
	}
}

func (h *Handler) handleMessage(remoteAddr string, msg IncomingMessage) control.State {
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
		return h.handleControlMessage(remoteAddr, msg)
	
	case MessageTypeSystem:
		return h.handleSystemMessage(remoteAddr, msg)
	
	default:
		return h.invalidState(ErrUnknownMessageType)
	}
}

func (h *Handler) handleControlMessage(remoteAddr string, msg IncomingMessage) control.State {
	if msg.Payload.Drive == nil {
		return h.invalidState(ErrMissingDrivePayload)
	}

	decision := h.accessManager.CanControl(remoteAddr)
	if !decision.Allowed {
		h.logger.Warn(
			"drive command rejected",
			"reason", decision.Reason,
			"remote_addr", remoteAddr,
			"client", decision.Client,
			"owner", decision.Owner,
		)

		state := h.control.State()
		state.LastCommandValid = false
		state.LastError = "control access denied: " + decision.Reason

		return state
	}

	h.logger.Info(
		"drive command accepted by access manager",
		"remote_addr", remoteAddr,
		"client", decision.Client,
		"owner", decision.Owner,
	)

	return h.control.Drive(control.DriveCommand{
		Left:  msg.Payload.Drive.Left,
		Right: msg.Payload.Drive.Right,
	})
}

func (h *Handler) handleSystemMessage(remoteAddr string, msg IncomingMessage) control.State {
	if msg.Payload.System == nil {
		return h.invalidState(ErrMissingSystemPayload)
	}

	switch msg.Payload.System.Command {
	case SystemCommandStop:
		decision := h.accessManager.CanControl(remoteAddr)
		if !decision.Allowed {
			h.logger.Warn(
				"stop command rejected",
				"reason", decision.Reason,
				"remote_addr", remoteAddr,
				"client", decision.Client,
				"owner", decision.Owner,
			)

			state := h.control.State()
			state.LastCommandValid = false
			state.LastError = "control access denied: " + decision.Reason

			return state
		}

		h.logger.Info(
			"stop command accepted by access manager",
			"remote_addr", remoteAddr,
			"client", decision.Client,
			"owner", decision.Owner,
		)

		return h.control.Stop()

	case SystemCommandEmergencyStop:
		decision := h.accessManager.AllowEmergencyStop(remoteAddr)
		if !decision.Allowed {
			h.logger.Warn(
				"emergency stop command rejected",
				"reason", decision.Reason,
				"remote_addr", remoteAddr,
				"client", decision.Client,
				"owner", decision.Owner,
			)

			state := h.control.State()
			state.LastCommandValid = false
			state.LastError = "emergency stop access denied: " + decision.Reason

			return state
		}

		h.logger.Warn(
			"emergency stop command accepted",
			"remote_addr", remoteAddr,
			"client", decision.Client,
			"owner", decision.Owner,
		)

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
