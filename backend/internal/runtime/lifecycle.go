package runtime

import (
	"context"
	"errors"
)

// CleanupStale runs at startup, once the worker lock is held and nothing else
// can be starting releases. Apps do not survive a Sprout restart: every
// container is removed, and release files are kept only for live deployments
// (which start again on demand). It returns how many resources it removed.
func CleanupStale(ctx context.Context, driver Driver, live map[string]bool) (int, error) {
	inventory, err := driver.List(ctx)
	if err != nil {
		return 0, err
	}
	var failures []error
	removed := 0
	for _, container := range inventory.Containers {
		if err := driver.Stop(ctx, container); err != nil {
			failures = append(failures, err)
			continue
		}
		removed++
	}
	for _, release := range inventory.Releases {
		if live[release.DeploymentID] {
			continue
		}
		if err := driver.Destroy(ctx, release); err != nil {
			failures = append(failures, err)
			continue
		}
		removed++
	}
	return removed, errors.Join(failures...)
}

// StopAll removes every running instance, keeping release files. It is the
// shutdown counterpart of CleanupStale and is best-effort: a crash skips it,
// which is why CleanupStale exists.
func StopAll(ctx context.Context, driver Driver) error {
	inventory, err := driver.List(ctx)
	if err != nil {
		return err
	}
	var failures []error
	for _, container := range inventory.Containers {
		failures = append(failures, driver.Stop(ctx, container))
	}
	return errors.Join(failures...)
}
