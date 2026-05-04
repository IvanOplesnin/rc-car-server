package ws

import (
	"log/slog"
	"net/http"

	"github.com/IvanOplesnin/rc-car-server.git/internal/control"
	"github.com/gorilla/websocket"
)

type Handler struct {
	logger  *slog.Logger
	control *control.Service
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
	// Когда будет VPN и постоянный адрес Raspberry Pi, это можно ограничить.
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
		"type", msg.Type,
		"left", msg.Left,
		"right", msg.Right,
	)

	switch msg.Type {
	case "drive":
		return h.control.Drive(control.DriveCommand{
			Left:  msg.Left,
			Right: msg.Right,
		})

	case "stop":
		return h.control.Stop()

	default:
		state := h.control.State()
		state.LastCommandValid = false
		state.LastError = ErrUnknownMessageType.Error()

		h.logger.Warn("unknown websocket message type", "type", msg.Type)

		return state
	}
}

func (h *Handler) sendState(conn *websocket.Conn, state control.State) {
	msg := StateMessage{
		Type:             "state",
		MotorConnected:   state.MotorConnected,
		CameraConnected:  state.CameraConnected,
		Left:             state.Left,
		Right:            state.Right,
		Failsafe:         state.Failsafe,
		LastCommandValid: state.LastCommandValid,
		Error:            state.LastError,
		BatteryVoltage:   state.BatteryVoltage,
		RSSI:             state.RSSI,
	}

	if err := conn.WriteJSON(msg); err != nil {
		h.logger.Error("write websocket state", "error", err)
	}
}
