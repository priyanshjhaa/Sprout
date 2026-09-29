package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/application"
	"github.com/priyanshjhaa/Sprout/backend/internal/config"
	"github.com/priyanshjhaa/Sprout/backend/internal/database"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
	"github.com/priyanshjhaa/Sprout/backend/internal/deployment"
	"github.com/priyanshjhaa/Sprout/backend/internal/httpapi"
	"github.com/priyanshjhaa/Sprout/backend/internal/identity"
	"github.com/priyanshjhaa/Sprout/backend/internal/sharing"
	"github.com/priyanshjhaa/Sprout/backend/internal/team"
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

	pool, err := database.Open(ctx, appConfig.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed")
		return 1
	}
	defer pool.Close()

	deploymentRepository := deployment.NewSQLRepository(pool)
	startupCtx, startupCancel := context.WithTimeout(ctx, 10*time.Second)
	releaseWorkerLock, err := deploymentRepository.AcquireProcessLock(startupCtx)
	startupCancel()
	if err != nil {
		logger.Error("simulation worker ownership or recovery failed; only one API process is supported")
		return 1
	}
	defer releaseWorkerLock()
	worker, err := deployment.NewManager(deploymentRepository, deployment.Simulate, logger, 2, 8, 15*time.Second)
	if err != nil {
		logger.Error("simulation worker configuration invalid")
		return 1
	}
	defer func() {
		if err := worker.Close(); err != nil {
			logger.Error("simulation shutdown incomplete; persisted jobs will be recovered on restart")
		}
	}()

	readiness := func(ctx context.Context) error {
		if err := deploymentRepository.CheckOwnership(ctx); err != nil {
			return err
		}
		if err := worker.Ready(); err != nil {
			return err
		}
		return database.Ping(ctx, pool)
	}

	queries := dbgen.New(pool)
	repository := application.NewSQLRepository(queries)
	service := application.NewService(repository)
	sharingService := sharing.NewService(queries)
	teamService := team.NewService(pool, identity.ClerkProfileProvider{})
	clerk.SetKey(appConfig.ClerkSecretKey)
	identityService := identity.NewService(identity.ClerkProfileProvider{}, identity.NewSQLRepository(pool))
	router := httpapi.NewRouter(logger, readiness, appConfig.WebOrigin)
	router.Route("/api/v1", func(api chi.Router) {
		api.Use(httpapi.AuthenticationMiddleware(identityService, logger, appConfig.WebOrigin))
		httpapi.RegisterIdentityRoutes(api, identityService, logger)
		httpapi.RegisterApplicationRoutes(api, service, logger)
		httpapi.RegisterSharingRoutes(api, sharingService, logger)
		httpapi.RegisterTeamRoutes(api, teamService, logger)
		httpapi.RegisterDeploymentSimulationRoutes(api, worker, logger)
	})

	return run(ctx, appConfig, logger, router)
}

func run(
	ctx context.Context,
	appConfig config.Config,
	logger *slog.Logger,
	handler http.Handler,
) int {
	listener, err := net.Listen("tcp", appConfig.APIAddress)
	if err != nil {
		logger.Error("listen failed", "error", err)
		return 1
	}

	server := httpapi.NewServer(listener.Addr().String(), handler)
	logger.Info("server listening", "service", serviceName, "address", listener.Addr().String())

	if err := httpapi.Serve(ctx, listener, server); err != nil {
		logger.Error("server stopped unexpectedly", "error", err)
		return 1
	}

	logger.Info("server stopped", "service", serviceName)

	return 0
}
