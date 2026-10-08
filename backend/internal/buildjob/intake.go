package buildjob

import (
	"context"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
	"github.com/priyanshjhaa/Sprout/backend/internal/deployment"
	"github.com/priyanshjhaa/Sprout/backend/internal/source"
)

// Worker is the part of the deployment manager that intake needs.
type Worker interface {
	CheckBuild(ctx context.Context, scope deployment.Scope) error
	SubmitBuild(ctx context.Context, scope deployment.Scope, source artifact.Descriptor) (deployment.Job, error)
}

// Intake accepts an uploaded source archive and queues a build of it.
type Intake struct {
	sources *artifact.Store
	worker  Worker
}

func NewIntake(sources *artifact.Store, worker Worker) *Intake {
	return &Intake{sources: sources, worker: worker}
}

// Check rejects a submission that would be refused anyway, before the caller
// spends time and disk receiving the upload.
func (i *Intake) Check(ctx context.Context, scope deployment.Scope) error {
	return i.worker.CheckBuild(ctx, scope)
}

// Submit stores the archive and queues it. If the worker does not accept the
// job, the stored archive is deleted before returning.
func (i *Intake) Submit(ctx context.Context, scope deployment.Scope, archive []byte) (deployment.Job, error) {
	if len(archive) == 0 || int64(len(archive)) > source.MaxArchiveBytes {
		return deployment.Job{}, deployment.ErrInvalid
	}
	stored, err := i.sources.Save(ctx, archive)
	if err != nil {
		return deployment.Job{}, deployment.ErrUnavailable
	}
	job, err := i.worker.SubmitBuild(ctx, scope, stored)
	if err != nil {
		// The request may already be cancelled; cleanup gets its own deadline.
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = i.sources.Delete(cleanup, stored)
		return deployment.Job{}, err
	}
	return job, nil
}

// Sweep reclaims files a crashed process never released. It must run after the
// worker lock is held and interrupted jobs are recovered, so nothing in flight
// can own them: every uploaded source goes, and only artifacts a deployment
// references stay.
func Sweep(ctx context.Context, sources, artifacts *artifact.Store, referenced map[string]bool) (int, error) {
	removed, err := sources.Sweep(ctx, func(string) bool { return false })
	if err != nil {
		return removed, err
	}
	more, err := artifacts.Sweep(ctx, func(id string) bool { return referenced[id] })
	return removed + more, err
}
