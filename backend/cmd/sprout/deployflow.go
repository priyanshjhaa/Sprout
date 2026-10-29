package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/apiclient"
	"github.com/priyanshjhaa/Sprout/backend/internal/sourcepack"
)

// The deploy flow shared by every front door in this binary: `sprout deploy`
// prints its progress, and `sprout mcp` will send it to a coding agent. The
// flow itself never prints; it tells a deployObserver what is happening.

var (
	// pollInterval is how often the flow asks how the build is going. Polling is
	// enough for one client watching one short build; the dashboard streams.
	pollInterval = time.Second
	// watchLimit is how long the flow waits for a build before giving up
	// watching. The server's own job limit is shorter.
	watchLimit = 10 * time.Minute
	// uploadTimeout bounds the upload of an archive of at most 8 MiB.
	uploadTimeout = 2 * time.Minute
)

// maxPollFailures is how many polls in a row may fail (a blip in the network
// or an API restart) before the flow gives up watching.
const maxPollFailures = 5

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// errStillRunning is returned when the flow stops watching a deployment that
// continues on the server.
var errStillRunning = errors.New("stopped watching; the deployment carries on in Sprout, so check the app in the dashboard")

// deployRequest is a checked request: an absolute path to an existing folder
// and a valid app slug.
type deployRequest struct {
	folder, slug, workspace string
}

// invalidRequest is a request that cannot be deployed as asked.
type invalidRequest struct{ message string }

func (e invalidRequest) Error() string { return e.message }

// newDeployRequest checks a folder and an optional app slug. Without a slug,
// the app is named after the folder, as in the dashboard.
func newDeployRequest(folder, app, workspace string) (deployRequest, error) {
	absolute, err := filepath.Abs(folder)
	if err != nil {
		return deployRequest{}, err
	}
	if info, err := os.Stat(absolute); err != nil || !info.IsDir() {
		return deployRequest{}, invalidRequest{fmt.Sprintf("%s is not a folder", folder)}
	}
	slug := app
	if slug == "" {
		if slug = slugify(filepath.Base(absolute)); slug == "" {
			return deployRequest{}, invalidRequest{"cannot make an app name from this folder's name; give the app a name like my-app"}
		}
	} else if !slugPattern.MatchString(slug) || len(slug) > 63 {
		return deployRequest{}, invalidRequest{"an app name must be lowercase letters, numbers and single dashes, like my-app"}
	}
	return deployRequest{folder: absolute, slug: slug, workspace: workspace}, nil
}

// deployObserver hears what the flow is doing, in order.
type deployObserver interface {
	packing(folder string)
	packed(archive sourcepack.Archive)
	appCreated(workspace, slug string)
	uploading(workspace, slug string)
	queued()
	stageFinished(stage string, succeeded bool)
}

// deployResult is a deployment that ended well: live, or built and stored on
// a server that does not run apps.
type deployResult struct {
	workspace  string
	app        apiclient.Application
	deployment apiclient.Deployment
	// url is the live address, or "" when it is not known.
	url  string
	took time.Duration
}

// refusedError means the server's own rules, run on this machine, refuse the
// folder, so nothing was uploaded.
type refusedError struct {
	code string
	// builtFolder is "dist", "build" or "out" when a finished website is there.
	builtFolder string
}

func (e refusedError) Error() string {
	return "not uploaded, because Sprout would refuse it: " + explain(e.code)
}

// failedError means the build ended without going live.
type failedError struct{ deployment apiclient.Deployment }

func (e failedError) Error() string {
	if e.deployment.Status == "cancelled" {
		return "the deployment was cancelled"
	}
	return "the deployment failed: " + explain(failureCode(e.deployment))
}

// deployFolder packs the folder, checks it against the server's own rules,
// finds or creates the app, uploads, and watches the build until it ends.
// Nothing is uploaded when the check already shows the server would refuse it.
func deployFolder(ctx context.Context, client *apiclient.Client, request deployRequest, observer deployObserver) (deployResult, error) {
	// 1. Pack and check on this machine, before anything is sent.
	observer.packing(request.folder)
	archive, err := sourcepack.Pack(ctx, request.folder)
	if err != nil {
		return deployResult{}, err
	}
	observer.packed(archive)
	if err := sourcepack.Check(ctx, archive); err != nil {
		if ctx.Err() != nil {
			return deployResult{}, ctx.Err()
		}
		return deployResult{}, refusedError{code: err.Error(), builtFolder: archive.BuiltFolder}
	}

	// 2. Find (or create) the app on the server.
	workspace, application, err := findApplication(ctx, client, request.workspace, request.slug, filepath.Base(request.folder), observer)
	if err != nil {
		return deployResult{}, err
	}

	// 3. Upload.
	observer.uploading(workspace, application.Slug)
	uploadCtx, cancel := context.WithTimeout(ctx, uploadTimeout)
	created, err := client.CreateDeployment(uploadCtx, workspace, application.ID, archive.Tar)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return deployResult{}, interrupted{"interrupted during the upload; if it finished, the deployment carries on in Sprout"}
		}
		return deployResult{}, describeUploadError(err)
	}

	// 4. Watch until it ends.
	started := time.Now()
	final, err := watch(ctx, client, workspace, application.ID, created, observer)
	if err != nil {
		return deployResult{}, err
	}
	result := deployResult{workspace: workspace, app: application, deployment: final, took: time.Since(started)}
	switch final.Status {
	case "live":
		ctx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()
		if latest, err := client.GetApplication(ctx, workspace, application.ID); err == nil && latest.URL != nil {
			result.url = *latest.URL
		}
		return result, nil
	case "succeeded":
		return result, nil
	}
	return result, failedError{deployment: final}
}

// slugify turns a folder name into an app slug: "My Site!" becomes "my-site".
func slugify(name string) string {
	var slug strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if dash && slug.Len() > 0 {
				slug.WriteByte('-')
			}
			slug.WriteRune(r)
			dash = false
			continue
		}
		dash = true
	}
	return strings.TrimRight(slug.String()[:min(slug.Len(), 63)], "-")
}

// findApplication resolves the workspace (the token owner's home one unless
// given) and finds the app by slug, creating it when it does not exist yet.
func findApplication(ctx context.Context, client *apiclient.Client, workspace, slug, name string, observer deployObserver) (string, apiclient.Application, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	capabilities, err := client.Capabilities(ctx)
	if err != nil {
		return "", apiclient.Application{}, err
	}
	if !capabilities.LocalBuilds {
		return "", apiclient.Application{}, errors.New("this Sprout API is not accepting deployments; start it with SPROUT_ENABLE_LOCAL_BUILDS=1")
	}
	if workspace == "" {
		me, err := client.Me(ctx)
		if err != nil {
			return "", apiclient.Application{}, err
		}
		workspace = me.Workspace.Slug
	}
	applications, err := client.ListApplications(ctx, workspace)
	if err != nil {
		return "", apiclient.Application{}, err
	}
	for _, application := range applications {
		if application.Slug != slug {
			continue
		}
		if application.Lifecycle != "active" {
			return "", apiclient.Application{}, fmt.Errorf("the app %q is %s; resume it in the dashboard to deploy", slug, application.Lifecycle)
		}
		return workspace, application, nil
	}
	application, err := client.CreateApplication(ctx, workspace, name, slug)
	if apiclient.HasCode(err, "application_slug_conflict") {
		return "", apiclient.Application{}, fmt.Errorf("an app called %q already exists in %s but you cannot see it; choose another app name", slug, workspace)
	}
	if err != nil {
		return "", apiclient.Application{}, err
	}
	observer.appCreated(workspace, slug)
	return workspace, application, nil
}

func describeUploadError(err error) error {
	switch {
	case apiclient.HasCode(err, "deployment_conflict"):
		return errors.New("this app already has a deployment in progress; wait for it to finish, or cancel it in the dashboard")
	case apiclient.HasCode(err, "deployment_queue_full"):
		return errors.New("Sprout is busy with other builds; try again in a minute")
	case apiclient.HasCode(err, "source_too_large"):
		return errors.New(explain("source_limit_exceeded"))
	}
	return err
}

// watch polls the deployment and reports each stage once, as it finishes.
func watch(ctx context.Context, client *apiclient.Client, workspace, applicationID string, deployment apiclient.Deployment, observer deployObserver) (apiclient.Deployment, error) {
	deadline := time.Now().Add(watchLimit)
	reported := map[string]bool{}
	announcedQueue := false
	failures := 0
	for {
		if deployment.Status == "queued" && !announcedQueue {
			observer.queued()
			announcedQueue = true
		}
		for _, stage := range deployment.Stages {
			if reported[stage.Name] || (stage.Status != "succeeded" && stage.Status != "failed") {
				continue
			}
			observer.stageFinished(stage.Name, stage.Status == "succeeded")
			reported[stage.Name] = true
		}
		if deployment.Done() {
			return deployment, nil
		}
		if time.Now().After(deadline) {
			return deployment, errStillRunning
		}
		select {
		case <-ctx.Done():
			return deployment, interrupted{errStillRunning.Error()}
		case <-time.After(pollInterval):
		}
		pollCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		next, err := client.GetDeployment(pollCtx, workspace, applicationID, deployment.ID)
		cancel()
		var apiErr *apiclient.Error
		switch {
		case err == nil:
			deployment, failures = next, 0
		case ctx.Err() != nil:
			return deployment, interrupted{errStillRunning.Error()}
		case errors.As(err, &apiErr) && apiErr.Code != "unexpected_response":
			return deployment, err // the server said no; asking again will not help
		default:
			if failures++; failures >= maxPollFailures {
				return deployment, fmt.Errorf("%w: %w", errStillRunning, err)
			}
		}
	}
}

// failureCode prefers the failed stage's code, which names the rule, over the
// deployment's own.
func failureCode(deployment apiclient.Deployment) string {
	for _, stage := range deployment.Stages {
		if stage.Status == "failed" && stage.FailureCode != nil {
			return *stage.FailureCode
		}
	}
	if deployment.FailureCode != nil {
		return *deployment.FailureCode
	}
	return "unknown"
}
