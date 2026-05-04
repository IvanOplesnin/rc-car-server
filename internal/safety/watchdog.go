package safety

import (
	"context"
	"log/slog"
	"time"

	"github.com/IvanOplesnin/rc-car-server.git/internal/control"
)

type ControlService interface {
	State() control.State
	Stop() control.State
}

type Watchdog struct {
	logger  *slog.Logger
	control ControlService
	timeout time.Duration
	period  time.Duration
}

func NewWatchdog(
	logger *slog.Logger,
	controlService ControlService,
	timeout time.Duration,
) *Watchdog {
	period := timeout / 2
	if period <= 0 {
		period = 100 * time.Millisecond
	}

	return &Watchdog{
		logger:  logger,
		control: controlService,
		timeout: timeout,
		period:  period,
	}
}

func (w *Watchdog) Run(ctx context.Context) {
	ticker := time.NewTicker(w.period)
	defer ticker.Stop()

	w.logger.Info(
		"safety watchdog started",
		"timeout", w.timeout.String(),
		"period", w.period.String(),
	)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("safety watchdog stopped")
			return

		case <-ticker.C:
			w.check()
		}
	}
}

func (w *Watchdog) check() {
	state := w.control.State()

	if state.LastCommandAt.IsZero() {
		return
	}

	if state.Left == 0 && state.Right == 0 {
		return
	}

	timeSinceLastCommand := time.Since(state.LastCommandAt)

	if timeSinceLastCommand <= w.timeout {
		return
	}

	w.logger.Warn(
		"command timeout exceeded, stopping motors",
		"last_command_age", timeSinceLastCommand.String(),
		"timeout", w.timeout.String(),
		"left", state.Left,
		"right", state.Right,
	)

	w.control.Stop()
}