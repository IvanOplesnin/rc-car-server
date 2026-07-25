package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	defaultListenAddress = "0.0.0.0:4211"
	defaultTargetAddress = "192.168.1.28:4211"
	defaultSourceIP      = "192.168.1.15"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("telemetry relay stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	listenAddress := envOrDefault("TELEMETRY_RELAY_LISTEN", defaultListenAddress)
	targetAddress := envOrDefault("TELEMETRY_RELAY_TARGET", defaultTargetAddress)
	allowedSource := net.ParseIP(envOrDefault("TELEMETRY_RELAY_SOURCE_IP", defaultSourceIP))
	if allowedSource == nil {
		return fmt.Errorf("invalid TELEMETRY_RELAY_SOURCE_IP")
	}

	listenAddr, err := net.ResolveUDPAddr("udp4", listenAddress)
	if err != nil {
		return fmt.Errorf("resolve listen address: %w", err)
	}

	targetAddr, err := net.ResolveUDPAddr("udp4", targetAddress)
	if err != nil {
		return fmt.Errorf("resolve target address: %w", err)
	}

	listener, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()

	forwarder, err := net.DialUDP("udp4", nil, targetAddr)
	if err != nil {
		return fmt.Errorf("connect to target: %w", err)
	}
	defer forwarder.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info(
		"telemetry relay started",
		"listen", listenAddr.String(),
		"target", targetAddr.String(),
		"allowed_source", allowedSource.String(),
	)

	buffer := make([]byte, 64*1024)
	packets := uint64(0)

	for {
		if err := listener.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}

		n, source, err := listener.ReadFromUDP(buffer)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				select {
				case <-ctx.Done():
					logger.Info("telemetry relay received shutdown signal")
					return nil
				default:
					continue
				}
			}

			return fmt.Errorf("read packet: %w", err)
		}

		if !source.IP.Equal(allowedSource) {
			logger.Warn("telemetry packet rejected", "source", source.String())
			continue
		}

		if _, err := forwarder.Write(buffer[:n]); err != nil {
			return fmt.Errorf("forward packet: %w", err)
		}

		packets++
		if packets == 1 || packets%120 == 0 {
			logger.Info("telemetry packets forwarded", "packets", packets)
		}
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
