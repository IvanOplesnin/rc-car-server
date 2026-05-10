package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IvanOplesnin/rc-car-server.git/internal/access"
	"github.com/IvanOplesnin/rc-car-server.git/internal/camera"
	"github.com/IvanOplesnin/rc-car-server.git/internal/config"
	"github.com/IvanOplesnin/rc-car-server.git/internal/control"
	"github.com/IvanOplesnin/rc-car-server.git/internal/httpserver"
	"github.com/IvanOplesnin/rc-car-server.git/internal/motor"
	"github.com/IvanOplesnin/rc-car-server.git/internal/safety"
	"github.com/IvanOplesnin/rc-car-server.git/internal/telemetry"
	"github.com/IvanOplesnin/rc-car-server.git/internal/ws"
)

type App struct {
	cfg               *config.Config
	logger            *slog.Logger
	httpServer        *httpserver.Server
	controlService    *control.Service
	motorClient       *motor.Client
	cameraProxy       *camera.Proxy
	cameraBroadcaster *camera.Broadcaster
	cameraMonitor     *camera.Monitor
	telemetryListener *telemetry.Listener
	wsHandler         *ws.Handler
	watchdog          *safety.Watchdog
	accessManager     *access.Manager
}

func New(cfg *config.Config, logger *slog.Logger) (*App, error) {
	motorClient, err := motor.NewClient(cfg.Motor.Address, logger)
	if err != nil {
		return nil, fmt.Errorf("create motor client: %w", err)
	}

	controlService := control.NewService(logger, motorClient)

	operators := make([]access.Operator, 0, len(cfg.Access.Operators))

	for _, operator := range cfg.Access.Operators {
		operators = append(operators, access.Operator{
			Name: operator.Name,
			IPs:  operator.IPs,
		})
	}

	accessManager := access.NewManager(
		logger,
		operators,
		time.Duration(cfg.Access.ControlTimeoutMS)*time.Millisecond,
	)

	wsHandler := ws.NewHandler(logger, controlService, accessManager)

	cameraProxy := camera.NewProxy(cfg.Camera.StreamURL, logger)
	cameraBroadcaster := camera.NewBroadcaster(cfg.Camera.StreamURL, logger, controlService)

	cameraMonitor := camera.NewMonitor(
		logger,
		cameraProxy,
		controlService,
		time.Duration(cfg.Camera.CheckIntervalMS)*time.Millisecond,
		time.Duration(cfg.Camera.CheckTimeoutMS)*time.Millisecond,
	)

	telemetryListener := telemetry.NewListener(
		logger,
		cfg.Telemetry.ListenAddress,
		time.Duration(cfg.Telemetry.MotorTimeoutMS)*time.Millisecond,
		controlService,
	)

	httpServer := httpserver.New(
		cfg,
		logger,
		wsHandler,
		cameraBroadcaster,
		controlService,
	)

	watchdogTimeout := time.Duration(cfg.Safety.CommandTimeoutMS) * time.Millisecond
	watchdog := safety.NewWatchdog(logger, controlService, watchdogTimeout)

	return &App{
		cfg:               cfg,
		logger:            logger,
		httpServer:        httpServer,
		controlService:    controlService,
		motorClient:       motorClient,
		cameraProxy:       cameraProxy,
		cameraBroadcaster: cameraBroadcaster,
		cameraMonitor:     cameraMonitor,
		telemetryListener: telemetryListener,
		wsHandler:         wsHandler,
		watchdog:          watchdog,
		accessManager:     accessManager,
	}, nil
}

func (a *App) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 2)

	go a.watchdog.Run(ctx)
	go a.cameraBroadcaster.Run(ctx)

	go func() {
		if err := a.telemetryListener.Run(ctx); err != nil {
			errCh <- fmt.Errorf("telemetry listener: %w", err)
		}
	}()

	go func() {
		errCh <- a.httpServer.Run()
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		cancel()
		return fmt.Errorf("app error: %w", err)

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
