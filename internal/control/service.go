package control

import (
	"log/slog"
	"sync"
	"time"

	"github.com/IvanOplesnin/rc-car-server.git/internal/motor"
)

type MotorClient interface {
	Send(left, right int) error
	Stop() error
}

type Service struct {
	logger *slog.Logger
	motor  MotorClient

	mu    sync.RWMutex
	state State
}

func NewService(logger *slog.Logger, motorClient MotorClient) *Service {
	return &Service{
		logger: logger,
		motor:  motorClient,
		state: State{
			MotorConnected:   false,
			CameraConnected:  false,
			Left:             0,
			Right:            0,
			Failsafe:         false,
			LastCommandValid: true,
			LastError:        "",
			LastCommandAt:    time.Time{},
		},
	}
}

func (s *Service) Drive(cmd DriveCommand) State {
	if err := validateDriveCommand(cmd); err != nil {
		return s.setInvalidCommand(err)
	}

	if err := s.motor.Send(cmd.Left, cmd.Right); err != nil {
		return s.setMotorError(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.MotorConnected = true
	s.state.Left = cmd.Left
	s.state.Right = cmd.Right
	s.state.LastCommandValid = true
	s.state.LastError = ""
	s.state.LastCommandAt = time.Now()

	s.logger.Info(
		"drive command accepted",
		"left", cmd.Left,
		"right", cmd.Right,
	)

	return s.state
}

func (s *Service) Stop() State {
	if err := s.motor.Stop(); err != nil {
		return s.setMotorError(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.MotorConnected = true
	s.state.Left = 0
	s.state.Right = 0
	s.state.LastCommandValid = true
	s.state.LastError = ""
	s.state.LastCommandAt = time.Now()

	s.logger.Info("stop command accepted")

	return s.state
}

func (s *Service) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.state
}

func (s *Service) SetCameraConnected(connected bool) State {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.CameraConnected != connected {
		s.logger.Info("camera connection state changed", "connected", connected)
	}

	s.state.CameraConnected = connected

	return s.state
}

func (s *Service) UpdateMotorTelemetry(t MotorTelemetry) State {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.state.MotorConnected {
		s.logger.Info("motor connection state changed", "connected", true)
	}

	s.state.MotorConnected = true
	s.state.BatteryVoltage = t.BatteryVoltage
	s.state.BatteryPercent = t.BatteryPercent
	s.state.RSSI = t.RSSI
	s.state.Left = t.Left
	s.state.Right = t.Right
	s.state.Failsafe = t.Failsafe
	s.state.UptimeMS = t.UptimeMS
	s.state.FreeHeap = t.FreeHeap
	s.state.LastTelemetryAt = time.Now()

	return s.state
}

func (s *Service) SetMotorConnected(connected bool) State {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.MotorConnected != connected {
		s.logger.Info("motor connection state changed", "connected", connected)
	}

	s.state.MotorConnected = connected

	return s.state
}

func (s *Service) setInvalidCommand(err error) State {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.LastCommandValid = false
	s.state.LastError = err.Error()

	s.logger.Warn("drive command rejected", "error", err)

	return s.state
}

func (s *Service) setMotorError(err error) State {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.MotorConnected = false
	s.state.LastCommandValid = false
	s.state.LastError = err.Error()

	s.logger.Error("send motor command", "error", err)

	return s.state
}




func validateDriveCommand(cmd DriveCommand) error {
	if cmd.Left < -100 || cmd.Left > 100 {
		return ErrInvalidLeftSpeed
	}

	if cmd.Right < -100 || cmd.Right > 100 {
		return ErrInvalidRightSpeed
	}

	return nil
}

var _ MotorClient = (*motor.Client)(nil)