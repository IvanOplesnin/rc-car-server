package camera

import (
	"context"
	"log/slog"
	"time"

	"github.com/IvanOplesnin/rc-car-server.git/internal/control"
)

type Checker interface {
	Check(ctx context.Context) error
}

type StateUpdater interface {
	SetCameraConnected(connected bool) control.State
}

type Monitor struct {
	logger   *slog.Logger
	checker  Checker
	state    StateUpdater
	interval time.Duration
	timeout  time.Duration
}

func NewMonitor(
	logger *slog.Logger,
	checker Checker,
	state StateUpdater,
	interval time.Duration,
	timeout time.Duration,
) *Monitor {
	return &Monitor{
		logger:   logger,
		checker:  checker,
		state:    state,
		interval: interval,
		timeout:  timeout,
	}
}

func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	m.logger.Info(
		"camera monitor started",
		"interval", m.interval.String(),
		"timeout", m.timeout.String(),
	)

	m.checkOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("camera monitor stopped")
			return

		case <-ticker.C:
			m.checkOnce(ctx)
		}
	}
}

func (m *Monitor) checkOnce(parentCtx context.Context) {
	ctx, cancel := context.WithTimeout(parentCtx, m.timeout)
	defer cancel()

	if err := m.checker.Check(ctx); err != nil {
		m.logger.Warn("camera check failed", "error", err)
		m.state.SetCameraConnected(false)
		return
	}

	m.state.SetCameraConnected(true)
}