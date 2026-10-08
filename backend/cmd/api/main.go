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
	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
	"github.com/priyanshjhaa/Sprout/backend/internal/buildjob"
	"github.com/priyanshjhaa/Sprout/backend/internal/config"
	"github.com/priyanshjhaa/Sprout/backend/internal/database"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
	"github.com/priyanshjhaa/Sprout/backend/internal/deployment"
	"github.com/priyanshjhaa/Sprout/backend/internal/httpapi"
	"github.com/priyanshjhaa/Sprout/backend/internal/identity"
	"github.com/priyanshjhaa/Sprout/backend/internal/nodeapp"
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
		logger.Error("deployment worker ownership or recovery failed; only one API process is supported")
		return 1
	}
	defer releaseWorkerLock()

	// Simulations always run. Real builds run untrusted code in Docker, so they
	// exist only when explicitly enabled for local development.
	var runner deployment.Runner = deployment.Staged(deployment.Simulate)
	jobTimeout := 15 * time.Second
	var sources, artifacts *artifact.Store
	if builds := appConfig.LocalBuilds; builds.Enabled {
		if sources, err = artifact.Open(builds.SourceDirectory); err != nil {
			logger.Error("SPROUT_SOURCE_DIR must be an existing private (0700) directory")
			return 1
		}
		defer sources.Close()
		if artifacts, err = artifact.Open(builds.ArtifactDirectory); err != nil {
			logger.Error("SPROUT_ARTIFACT_DIR must be an existing private (0700) directory")
			return 1
		}
		defer artifacts.Close()
		builder, err := nodeapp.NewRunner(nodeapp.DefaultImage, 2*time.Minute)
		if err != nil {
			logger.Error("node build runner configuration invalid")
			return 1
		}
		// The worker lock is held and interrupted jobs are recovered, so nothing
		// in flight can own a stored file: reclaim what a crash left behind.
		sweepCtx, sweepCancel := context.WithTimeout(ctx, 30*time.Second)
		referenced, err := deploymentRepository.ReferencedArtifacts(sweepCtx)
		if err == nil {
			var removed int
			removed, err = buildjob.Sweep(sweepCtx, sources, artifacts, referenced)
			logger.Info("build storage swept", "removed", removed)
		}
		sweepCancel()
		if err != nil {
			logger.Error("build storage sweep failed")
			return 1
		}
		runner = buildjob.NewRunner(sources, artifacts, builder, runner, logger)
		jobTimeout = 3 * time.Minute
		logger.Warn("local builds enabled: uploaded code runs in Docker on this machine")
	}
	worker, err := deployment.NewManager(deploymentRepository, runner, logger, 2, 8, jobTimeout)
	if err != nil {
		logger.Error("deployment worker configuration invalid")
		return 1
	}
	defer func() {
		if err := worker.Close(); err != nil {
			logger.Error("deployment worker shutdown incomplete; persisted jobs will be recovered on restart")
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
		httpapi.RegisterDeploymentStreamRoutes(api, worker, logger, ctx)
		if appConfig.LocalBuilds.Enabled {
			httpapi.RegisterDeploymentRoutes(api, worker, buildjob.NewIntake(sources, worker), logger)
			httpapi.RegisterBuildStreamRoutes(api, worker, logger, ctx)
		}
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
