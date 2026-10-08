// Package buildjob runs real deployment builds on the deployment worker: it
// loads an uploaded source archive, prepares and validates it, builds it in the
// sealed Node container, and stores the output as an artifact. It never starts
// the application; a succeeded build means "built", not "live".
package buildjob

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log/slog"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
	"github.com/priyanshjhaa/Sprout/backend/internal/deployment"
	"github.com/priyanshjhaa/Sprout/backend/internal/nodeapp"
	"github.com/priyanshjhaa/Sprout/backend/internal/source"
)

// Builder turns a prepared source tree into a bounded output archive.
// *nodeapp.Runner implements it; tests substitute a fake.
type Builder interface {
	BuildFiles(ctx context.Context, files fs.FS) ([]byte, error)
}

// Runner is the deployment.Runner for this process. Simulations are delegated
// unchanged; real builds use the stores and builder.
type Runner struct {
	sources, artifacts *artifact.Store
	builder            Builder
	simulation         deployment.Runner
	logger             *slog.Logger
}

func NewRunner(sources, artifacts *artifact.Store, builder Builder, simulation deployment.Runner, logger *slog.Logger) *Runner {
	return &Runner{sources: sources, artifacts: artifacts, builder: builder, simulation: simulation, logger: logger}
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
	return outcome, steps.Complete("package")
}

// Release deletes the uploaded source, which is never needed after a build,
// and the artifact unless the deployment now references it.
func (r *Runner) Release(ctx context.Context, work deployment.Work, outcome deployment.Outcome, kept bool) {
	if work.Simulated {
		r.simulation.Release(ctx, work, outcome, kept)
		return
	}
	if work.Source != nil {
		r.remove(ctx, r.sources, *work.Source, "source", work.DeploymentID)
	}
	if outcome.Artifact != nil && !kept {
		r.remove(ctx, r.artifacts, *outcome.Artifact, "artifact", work.DeploymentID)
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
