// Package buildjob runs real deployments on the deployment worker: it loads an
// uploaded source archive, prepares and validates it, builds it in the sealed
// Node container, stores the output as an artifact, then provisions, starts and
// health-checks the release through the runtime driver. Only a release that
// answers /health is reported live.
package buildjob

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
	"github.com/priyanshjhaa/Sprout/backend/internal/deployment"
	"github.com/priyanshjhaa/Sprout/backend/internal/nodeapp"
	rt "github.com/priyanshjhaa/Sprout/backend/internal/runtime"
	"github.com/priyanshjhaa/Sprout/backend/internal/source"
)

// Builder turns a prepared source tree into a bounded output archive.
// *nodeapp.Runner implements it; tests substitute a fake.
type Builder interface {
	BuildFiles(ctx context.Context, files fs.FS) ([]byte, error)
}

// HealthCheck waits until the instance at endpoint is serving, or fails.
type HealthCheck func(ctx context.Context, endpoint string) error

// DefaultHealthCheck allows an app thirty seconds to answer GET /health.
func DefaultHealthCheck(ctx context.Context, endpoint string) error {
	return rt.WaitHealthy(ctx, endpoint, 30*time.Second)
}

// Runner is the deployment.Runner for this process. Simulations are delegated
// unchanged; real builds use the stores, builder and runtime driver.
type Runner struct {
	sources, artifacts *artifact.Store
	builder            Builder
	driver             rt.Driver
	healthy            HealthCheck
	simulation         deployment.Runner
	logger             *slog.Logger
}

func NewRunner(sources, artifacts *artifact.Store, builder Builder, driver rt.Driver, healthy HealthCheck, simulation deployment.Runner, logger *slog.Logger) *Runner {
	return &Runner{sources: sources, artifacts: artifacts, builder: builder, driver: driver, healthy: healthy, simulation: simulation, logger: logger}
}

func (r *Runner) Run(ctx context.Context, work deployment.Work, steps deployment.Steps) (deployment.Outcome, error) {
	if work.Simulated {
		return r.simulation.Run(ctx, work, steps)
	}
	if work.Source == nil {
		return deployment.Outcome{}, &deployment.Failure{Code: "source_missing"}
	}
	if err := steps.Begin("source"); err != nil {
		return deployment.Outcome{}, err
	}
	data, err := r.sources.Load(ctx, *work.Source)
	if err != nil {
		return deployment.Outcome{}, classify(err)
	}
	// The prepared tree exists only inside this callback, so the source and
	// build stages both run within it; the tree is removed when it returns.
	var output []byte
	_, err = source.WithArchive(ctx, bytes.NewReader(data), func(ctx context.Context, files fs.FS) error {
		if err := nodeapp.Validate(ctx, files); err != nil {
			return err
		}
		if err := steps.Complete("source"); err != nil {
			return err
		}
		if err := steps.Begin("build"); err != nil {
			return err
		}
		built, err := r.builder.BuildFiles(ctx, files)
		if err != nil {
			return err
		}
		output = built
		return steps.Complete("build")
	})
	if err != nil {
		return deployment.Outcome{}, classify(err)
	}
	if err := steps.Begin("package"); err != nil {
		return deployment.Outcome{}, err
	}
	stored, err := r.artifacts.Save(ctx, output)
	if err != nil {
		return deployment.Outcome{}, classify(err)
	}
	// From here the artifact exists, so it is always returned: if anything
	// below fails, Release sees it and deletes it.
	outcome := deployment.Outcome{Artifact: &stored}
	if err := steps.Complete("package"); err != nil {
		return outcome, err
	}
	return r.goLive(ctx, work, output, outcome, steps)
}

// goLive provisions the release, starts it, and waits for it to be healthy.
// Provisioned is set before the first driver call so Release cleans up even
// when a call fails halfway.
func (r *Runner) goLive(ctx context.Context, work deployment.Work, archive []byte, outcome deployment.Outcome, steps deployment.Steps) (deployment.Outcome, error) {
	release := rt.Release{ApplicationID: work.ApplicationID, DeploymentID: work.DeploymentID}
	if err := steps.Begin("provision"); err != nil {
		return outcome, err
	}
	outcome.Provisioned = true
	if err := r.driver.Prepare(ctx, release, archive); err != nil {
		return outcome, runtimeFailure("provision_failed", err)
	}
	if err := steps.Complete("provision"); err != nil {
		return outcome, err
	}
	if err := steps.Begin("release"); err != nil {
		return outcome, err
	}
	instance, err := r.driver.Start(ctx, release)
	if err != nil {
		return outcome, runtimeFailure("release_failed", err)
	}
	if err := steps.Complete("release"); err != nil {
		return outcome, err
	}
	if err := steps.Begin("health_check"); err != nil {
		return outcome, err
	}
	if err := r.healthy(ctx, instance.Endpoint); err != nil {
		if ctx.Err() != nil {
			return outcome, ctx.Err()
		}
		return outcome, &deployment.Failure{Code: "health_check_failed", Err: err}
	}
	if err := steps.Complete("health_check"); err != nil {
		return outcome, err
	}
	outcome.Live = true
	return outcome, nil
}

// runtimeFailure keeps "the runtime is down" distinct from "this release failed".
func runtimeFailure(code string, err error) error {
	if errors.Is(err, rt.ErrUnavailable) {
		code = "runtime_unavailable"
	}
	return &deployment.Failure{Code: code, Err: err}
}

// Release deletes the uploaded source, which is never needed after a build.
// Unless the deployment went live, it also removes the runtime resources and
// the artifact; if it did, the application's earlier releases are retired.
func (r *Runner) Release(ctx context.Context, work deployment.Work, outcome deployment.Outcome, kept bool) {
	if work.Simulated {
		r.simulation.Release(ctx, work, outcome, kept)
		return
	}
	if work.Source != nil {
		r.remove(ctx, r.sources, *work.Source, "source", work.DeploymentID)
	}
	if outcome.Provisioned {
		release := rt.Release{ApplicationID: work.ApplicationID, DeploymentID: work.DeploymentID}
		if kept {
			r.retireSuperseded(ctx, release)
		} else if err := r.driver.Destroy(ctx, release); err != nil {
			// The startup cleanup reclaims anything left here.
			r.logger.Error("release cleanup failed", "deployment_id", work.DeploymentID)
		}
	}
	if outcome.Artifact != nil && !kept {
		r.remove(ctx, r.artifacts, *outcome.Artifact, "artifact", work.DeploymentID)
	}
}

// retireSuperseded stops the application's previous releases now that this one
// is live. Their artifacts stay, so they remain candidates for rollback.
func (r *Runner) retireSuperseded(ctx context.Context, live rt.Release) {
	inventory, err := r.driver.List(ctx)
	if err != nil {
		r.logger.Error("could not list superseded releases", "deployment_id", live.DeploymentID)
		return
	}
	seen := map[string]bool{live.DeploymentID: true}
	for _, old := range append(inventory.Containers, inventory.Releases...) {
		if old.ApplicationID != live.ApplicationID || seen[old.DeploymentID] {
			continue
		}
		seen[old.DeploymentID] = true
		if err := r.driver.Destroy(ctx, old); err != nil {
			r.logger.Error("superseded release cleanup failed", "deployment_id", old.DeploymentID)
		}
	}
}

func (r *Runner) remove(ctx context.Context, store *artifact.Store, blob artifact.Descriptor, kind, deploymentID string) {
	if err := store.Delete(ctx, blob); err != nil && !errors.Is(err, artifact.ErrMissing) {
		// The startup sweep reclaims anything left here.
		r.logger.Error("build cleanup failed", "kind", kind, "deployment_id", deploymentID)
	}
}

// knownFailures are errors whose text is already a stable, user-safe code.
var knownFailures = []error{
	source.ErrInvalid, source.ErrLimit, source.ErrSensitive, source.ErrStorage, source.ErrCleanup,
	nodeapp.ErrContract, nodeapp.ErrImage, nodeapp.ErrDependencies, nodeapp.ErrBuild,
	nodeapp.ErrArtifact, nodeapp.ErrArtifactSize, nodeapp.ErrDocker, nodeapp.ErrCleanup,
	artifact.ErrStorage,
}

// Runtime failures are classified where they happen (runtimeFailure and
// goLive), because the same driver error means different things per stage.

// classify gives known failures their code. Anything else, including progress
// persistence errors and cancellation, passes through for the manager to map.
func classify(err error) error {
	if errors.Is(err, artifact.ErrMissing) || errors.Is(err, artifact.ErrCorrupt) {
		return &deployment.Failure{Code: "source_unavailable", Err: err}
	}
	for _, known := range knownFailures {
		if errors.Is(err, known) {
			return &deployment.Failure{Code: known.Error(), Err: err}
		}
	}
	return err
}
