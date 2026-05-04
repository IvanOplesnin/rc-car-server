package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IvanOplesnin/rc-car-server.git/internal/camera"
	"github.com/IvanOplesnin/rc-car-server.git/internal/config"
	"github.com/IvanOplesnin/rc-car-server.git/internal/control"
	"github.com/IvanOplesnin/rc-car-server.git/internal/httpserver"
	"github.com/IvanOplesnin/rc-car-server.git/internal/motor"
	"github.com/IvanOplesnin/rc-car-server.git/internal/safety"
	"github.com/IvanOplesnin/rc-car-server.git/internal/ws"
)

type App struct {
	cfg            *config.Config
	logger         *slog.Logger
	httpServer     *httpserver.Server
	controlService *control.Service
	motorClient    *motor.Client
	cameraProxy    *camera.Proxy
	cameraMonitor  *camera.Monitor
	wsHandler      *ws.Handler
	watchdog       *safety.Watchdog
}

func New(cfg *config.Config, logger *slog.Logger) (*App, error) {
	motorClient, err := motor.NewClient(cfg.Motor.Address, logger)
	if err != nil {
		return nil, fmt.Errorf("create motor client: %w", err)
	}

	controlService := control.NewService(logger, motorClient)
	wsHandler := ws.NewHandler(logger, controlService)

	cameraProxy := camera.NewProxy(cfg.Camera.StreamURL, logger)

	cameraMonitor := camera.NewMonitor(
		logger,
		cameraProxy,
		controlService,
		time.Duration(cfg.Camera.CheckIntervalMS)*time.Millisecond,
		time.Duration(cfg.Camera.CheckTimeoutMS)*time.Millisecond,
	)

	httpServer := httpserver.New(
		cfg,
		logger,
		wsHandler,
		cameraProxy,
		controlService,
	)

	watchdogTimeout := time.Duration(cfg.Safety.CommandTimeoutMS) * time.Millisecond
	watchdog := safety.NewWatchdog(logger, controlService, watchdogTimeout)

	return &App{
		cfg:            cfg,
		logger:         logger,
		httpServer:     httpServer,
		controlService: controlService,
		motorClient:    motorClient,
		cameraProxy:    cameraProxy,
		cameraMonitor:  cameraMonitor,
		wsHandler:      wsHandler,
		watchdog:       watchdog,
	}, nil
}

func (a *App) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)

	go a.watchdog.Run(ctx)
	go a.cameraMonitor.Run(ctx)

	go func() {
		errCh <- a.httpServer.Run()
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		cancel()
		return fmt.Errorf("http server error: %w", err)

	case sig := <-stopCh:
		a.logger.Info("received shutdown signal", "signal", sig.String())

		cancel()

		a.controlService.Stop()

		ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()

		if err := a.httpServer.Shutdown(ctxShutdown); err != nil {
			return fmt.Errorf("shutdown app: %w", err)
		}

		if err := a.motorClient.Close(); err != nil {
			a.logger.Error("close motor client", "error", err)
		}

		a.logger.Info("app stopped")
		return nil
	}
}