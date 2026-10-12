// Package runtime starts built releases as running applications. Sprout's
// control plane talks to a Driver; where the application actually runs (local
// Docker today, a cloud host later) is the driver's concern. Applications are
// untrusted: a driver must never run them on the machine holding Sprout's
// database and secrets.
package runtime

import (
	"context"
	"errors"
)

var (
	ErrInvalid     = errors.New("runtime_release_invalid")
	ErrUnavailable = errors.New("runtime_unavailable")
	ErrStart       = errors.New("runtime_start_failed")
)

// Release names one immutable built deployment. Both IDs are UUIDs; drivers
// derive resource names from them, so they are validated before use.
type Release struct {
	ApplicationID string
	DeploymentID  string
}

// Instance is a running release and where to reach it from the control plane.
type Instance struct {
	Release  Release
	Endpoint string // host:port, loopback for the local driver
}

// Inventory is everything a driver currently holds for Sprout, so a
// reconciler can compare it with what should exist.
type Inventory struct {
	Containers []Release // started instances, running or not
	Releases   []Release // prepared release files
}

// Driver turns a release archive into a running application and back.
type Driver interface {
	// Prepare stores the release files. The archive is Sprout's own validated
	// repacking of a build output, never raw user input.
	Prepare(ctx context.Context, release Release, archive []byte) error
	// Start runs a prepared release and returns where it listens. It does not
	// wait for health; the caller checks /health.
	Start(ctx context.Context, release Release) (Instance, error)
	// Stop removes the running instance but keeps the prepared files.
	Stop(ctx context.Context, release Release) error
	// Destroy removes the instance and the prepared files. Missing parts are fine.
	Destroy(ctx context.Context, release Release) error
	// List reports every resource this driver owns.
	List(ctx context.Context) (Inventory, error)
}
