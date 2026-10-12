package runtime

import (
	"context"
	"slices"
	"testing"
)

type inventoryDriver struct {
	Driver
	inventory     Inventory
	stopped, gone []Release
}

func (d *inventoryDriver) List(context.Context) (Inventory, error) { return d.inventory, nil }
func (d *inventoryDriver) Stop(_ context.Context, r Release) error {
	d.stopped = append(d.stopped, r)
	return nil
}
func (d *inventoryDriver) Destroy(_ context.Context, r Release) error {
	d.gone = append(d.gone, r)
	return nil
}

func TestCleanupStaleKeepsOnlyLiveReleaseFiles(t *testing.T) {
	live := Release{ApplicationID: testRelease.ApplicationID, DeploymentID: "33333333-3333-4333-8333-333333333333"}
	d := &inventoryDriver{inventory: Inventory{Containers: []Release{testRelease, live}, Releases: []Release{testRelease, live}}}
	removed, err := CleanupStale(context.Background(), d, map[string]bool{live.DeploymentID: true})
	if err != nil || removed != 3 {
		t.Fatalf("removed %d: %v", removed, err)
	}
	if !slices.Equal(d.stopped, []Release{testRelease, live}) {
		t.Fatalf("every container must stop, got %v", d.stopped)
	}
	if !slices.Equal(d.gone, []Release{testRelease}) {
		t.Fatalf("only the non-live release is destroyed, got %v", d.gone)
	}
	d.stopped = nil
	if err := StopAll(context.Background(), d); err != nil || len(d.stopped) != 2 {
		t.Fatalf("StopAll stopped %v: %v", d.stopped, err)
	}
}
