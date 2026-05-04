package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/IvanOplesnin/rc-car-server.git/internal/control"
)

type MotorTelemetryMessage struct {
	Type           string  `json:"type"`
	BatteryVoltage float64 `json:"battery_voltage"`
	RSSI           int     `json:"rssi"`
	Left           int     `json:"left"`
	Right          int     `json:"right"`
	Failsafe       bool    `json:"failsafe"`
}

type StateUpdater interface {
	UpdateMotorTelemetry(t control.MotorTelemetry) control.State
	SetMotorConnected(connected bool) control.State
	State() control.State
}

type Listener struct {
	logger       *slog.Logger
	listenAddr   string
	motorTimeout time.Duration
	state        StateUpdater
}

func NewListener(
	logger *slog.Logger,
	listenAddr string,
	motorTimeout time.Duration,
	state StateUpdater,
) *Listener {
	return &Listener{
		logger:       logger,
		listenAddr:   listenAddr,
		motorTimeout: motorTimeout,
		state:        state,
	}
}

func (l *Listener) Run(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", l.listenAddr)
	if err != nil {
		return fmt.Errorf("resolve telemetry udp address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen telemetry udp: %w", err)
	}
	defer conn.Close()

	l.logger.Info("telemetry listener started", "address", l.listenAddr)

	errCh := make(chan error, 1)

	go func() {
		errCh <- l.readLoop(ctx, conn)
	}()

	go l.connectionWatchdog(ctx)

	select {
	case <-ctx.Done():
		l.logger.Info("telemetry listener stopped")
		return nil

	case err := <-errCh:
		return err
	}
}

func (l *Listener) readLoop(ctx context.Context, conn *net.UDPConn) error {
	buf := make([]byte, 2048)

	for {
		select {
		case <-ctx.Done():
			return nil

		default:
			if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
				return fmt.Errorf("set telemetry read deadline: %w", err)
			}

			n, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}

				return fmt.Errorf("read telemetry udp: %w", err)
			}

			l.handlePacket(remoteAddr, buf[:n])
		}
	}
}

func (l *Listener) handlePacket(remoteAddr *net.UDPAddr, data []byte) {
	var msg MotorTelemetryMessage

	if err := json.Unmarshal(data, &msg); err != nil {
		l.logger.Warn(
			"invalid telemetry json",
			"remote_addr", remoteAddr.String(),
			"error", err,
			"data", string(data),
		)
		return
	}

	if msg.Type != "motor_status" {
		l.logger.Warn(
			"unknown telemetry message type",
			"remote_addr", remoteAddr.String(),
			"type", msg.Type,
		)
		return
	}

	state := l.state.UpdateMotorTelemetry(control.MotorTelemetry{
		BatteryVoltage: msg.BatteryVoltage,
		RSSI:           msg.RSSI,
		Left:           msg.Left,
		Right:          msg.Right,
		Failsafe:       msg.Failsafe,
	})

	l.logger.Info(
		"motor telemetry received",
		"remote_addr", remoteAddr.String(),
		"battery_voltage", state.BatteryVoltage,
		"rssi", state.RSSI,
		"left", state.Left,
		"right", state.Right,
		"failsafe", state.Failsafe,
	)
}

func (l *Listener) connectionWatchdog(ctx context.Context) {
	ticker := time.NewTicker(l.motorTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			state := l.state.State()

			if state.LastTelemetryAt.IsZero() {
				l.state.SetMotorConnected(false)
				continue
			}

			if time.Since(state.LastTelemetryAt) > l.motorTimeout {
				l.logger.Warn(
					"motor telemetry timeout",
					"last_telemetry_age", time.Since(state.LastTelemetryAt).String(),
					"timeout", l.motorTimeout.String(),
				)

				l.state.SetMotorConnected(false)
			}
		}
	}
}