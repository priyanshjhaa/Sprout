package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/priyanshjhaa/Sprout/backend/internal/config"
	"github.com/priyanshjhaa/Sprout/backend/internal/httpapi"
)

const serviceName = "sprout-api"

func main() {
	os.Exit(realMain())
}

func realMain() int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	appConfig, err := config.Load(os.LookupEnv)
	if err != nil {
		logger.Error("configuration invalid", "error", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return run(ctx, appConfig, logger)
}

func run(ctx context.Context, appConfig config.Config, logger *slog.Logger) int {
	listener, err := net.Listen("tcp", appConfig.APIAddress)
	if err != nil {
		logger.Error("listen failed", "error", err)
		return 1
	}

	server := httpapi.NewServer(listener.Addr().String(), logger, httpapi.AlwaysReady)
	logger.Info("server listening", "service", serviceName, "address", listener.Addr().String())

	if err := httpapi.Serve(ctx, listener, server); err != nil {
		logger.Error("server stopped unexpectedly", "error", err)
		return 1
	}

	logger.Info("server stopped", "service", serviceName)

	return 0
}
