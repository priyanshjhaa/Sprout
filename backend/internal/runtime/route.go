package runtime

import (
	"context"
	"errors"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
)

// ErrNoApp means no application exists at the requested workspace and slug.
var ErrNoApp = errors.New("runtime_app_not_found")

// Route is what the public proxy needs to know to serve one application: its
// lifecycle and, if it has one, the live release and where its files came from.
type Route struct {
	Release   Release
	Lifecycle string // active, paused or archived
	Live      bool
	Artifact  artifact.Descriptor
}

// Lookup finds an application by workspace and application slug. Live apps are
// public, so it performs no permission check.
type Lookup func(ctx context.Context, workspaceSlug, applicationSlug string) (Route, error)
