package runtime

import (
	"context"
	"errors"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
)

// ErrNoApp means no application exists at the requested workspace and slug.
var ErrNoApp = errors.New("runtime_app_not_found")

// How a release is served. Node releases run in a container; static releases
// are files read straight from the stored artifact and never execute.
const (
	RuntimeNode   = "node"
	RuntimeStatic = "static"
)

// Route is what the public proxy needs to know to serve one application: its
// lifecycle and, if it has one, the live release and where its files came from.
type Route struct {
	Release   Release
	Lifecycle string // active, paused or archived
	Live      bool
	Runtime   string // RuntimeNode or RuntimeStatic
	Artifact  artifact.Descriptor
}

// Lookup finds an application by workspace and application slug. Live apps are
// public, so it performs no permission check.
type Lookup func(ctx context.Context, workspaceSlug, applicationSlug string) (Route, error)
