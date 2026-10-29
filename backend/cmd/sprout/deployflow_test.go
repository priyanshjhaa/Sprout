package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/priyanshjhaa/Sprout/backend/internal/apiclient"
	"github.com/priyanshjhaa/Sprout/backend/internal/sourcepack"
)

// recorder is a deployObserver that writes down what it hears.
type recorder struct{ events []string }

func (r *recorder) packing(string) { r.events = append(r.events, "packing") }
func (r *recorder) packed(archive sourcepack.Archive) {
	r.events = append(r.events, fmt.Sprintf("packed %s %d", archive.Kind, archive.Files))
}
func (r *recorder) appCreated(_, slug string) { r.events = append(r.events, "created "+slug) }
func (r *recorder) uploading(_, slug string)  { r.events = append(r.events, "uploading "+slug) }
func (r *recorder) queued()                   { r.events = append(r.events, "queued") }
func (r *recorder) stageFinished(stage string, ok bool) {
	r.events = append(r.events, fmt.Sprintf("%s %v", stage, ok))
}

func TestDeployFlowReportsEachStepInOrder(t *testing.T) {
	_, api := newFakeSprout(t)
	deployHarness(t, api) // fast polling for this test
	client, err := apiclient.New(api.URL, goodToken, userAgent)
	if err != nil {
		t.Fatal(err)
	}
	request, err := newDeployRequest(site(t, "Flow Site", map[string]string{"index.html": "x"}), "", "")
	if err != nil {
		t.Fatal(err)
	}
	var heard recorder
	result, err := deployFolder(context.Background(), client, request, &heard)
	if err != nil {
		t.Fatal(err)
	}
	want := "packing, packed static 1, created flow-site, uploading flow-site, queued, " +
		"source true, build true, package true, provision true, release true, health_check true"
	if got := strings.Join(heard.events, ", "); got != want {
		t.Fatalf("events\n got: %s\nwant: %s", got, want)
	}
	if result.url != "http://flow-site.ada-team.localhost:8081" || result.deployment.Status != "live" || result.workspace != "ada-team" {
		t.Fatalf("unexpected result %+v", result)
	}
}

func TestDeployFlowRefusalSaysWhyAndUploadsNothing(t *testing.T) {
	fake, api := newFakeSprout(t)
	client, _ := apiclient.New(api.URL, goodToken, userAgent)
	request, err := newDeployRequest(site(t, "shop", map[string]string{
		"package.json":    `{"name":"shop","private":true,"dependencies":{"express":"^5.0.0"}}`,
		"dist/index.html": "x",
	}), "", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = deployFolder(context.Background(), client, request, &recorder{})
	var refused refusedError
	if !errors.As(err, &refused) || refused.code != "node_build_dependencies_unsupported" || refused.builtFolder != "dist" {
		t.Fatalf("expected a refusal naming the rule and dist, got %v", err)
	}
	if len(fake.uploads) != 0 {
		t.Fatal("a refused folder was uploaded")
	}
}

func TestNewDeployRequestChecksItsInput(t *testing.T) {
	folder := site(t, "ok", map[string]string{"index.html": "x"})
	for name, tc := range map[string]struct{ folder, app string }{
		"missing folder": {folder + "/nope", ""},
		"bad app name":   {folder, "Bad Name"},
		"long app name":  {folder, strings.Repeat("a", 64)},
	} {
		var invalid invalidRequest
		if _, err := newDeployRequest(tc.folder, tc.app, ""); !errors.As(err, &invalid) {
			t.Errorf("%s: expected invalidRequest, got %v", name, err)
		}
	}
	request, err := newDeployRequest(folder, "", "team")
	if err != nil || request.slug != "ok" || request.workspace != "team" || !strings.HasPrefix(request.folder, "/") {
		t.Fatalf("request %+v, err %v", request, err)
	}
}
