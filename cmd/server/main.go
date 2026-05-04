package main

import (
	"log/slog"
	"os"

	"github.com/IvanOplesnin/rc-car-server.git/internal/app"
	"github.com/IvanOplesnin/rc-car-server.git/internal/config"
)

func main() {
	logger := slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}),
	)

	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("create app", "error", err)
		os.Exit(1)
	}

	if err := application.Run(); err != nil {
		logger.Error("run app", "error", err)
		os.Exit(1)
	}
}