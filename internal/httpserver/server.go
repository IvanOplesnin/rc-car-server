package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/IvanOplesnin/rc-car-server.git/internal/config"
	"github.com/IvanOplesnin/rc-car-server.git/internal/control"
)

type StateProvider interface {
	State() control.State
}

type Server struct {
	cfg           *config.Config
	logger        *slog.Logger
	wsHandler     http.Handler
	cameraHandler http.Handler
	stateProvider StateProvider
	server        *http.Server
}

func New(
	cfg *config.Config,
	logger *slog.Logger,
	wsHandler http.Handler,
	cameraHandler http.Handler,
	stateProvider StateProvider,
) *Server {
	s := &Server{
		cfg:           cfg,
		logger:        logger,
		wsHandler:     wsHandler,
		cameraHandler: cameraHandler,
		stateProvider: stateProvider,
	}

	mux := http.NewServeMux()

	s.registerRoutes(mux)

	s.server = &http.Server{
		Addr:              cfg.HTTPAddress(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s
}

func (s *Server) Run() error {
	s.logger.Info("starting http server", "address", s.cfg.HTTPAddress())

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen and serve: %w", err)
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("stopping http server")

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	return nil
}