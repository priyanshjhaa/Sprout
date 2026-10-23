package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/apiclient"
	"github.com/priyanshjhaa/Sprout/backend/internal/sourcepack"
)

const deployUsage = "usage: sprout deploy [folder] [--app slug] [--workspace slug]"

var (
	// pollInterval is how often the CLI asks how the build is going. Polling is
	// enough for one terminal watching one short build; the dashboard streams.
	pollInterval = time.Second
	// watchLimit is how long the CLI waits for a build before giving up
	// watching. The server's own job limit is shorter.
	watchLimit = 10 * time.Minute
	// uploadTimeout bounds the upload of an archive of at most 8 MiB.
	uploadTimeout = 2 * time.Minute
)

// maxPollFailures is how many polls in a row may fail (a blip in the network
// or an API restart) before the CLI gives up watching.
const maxPollFailures = 5

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// errStillRunning is returned when the CLI stops watching a deployment that
// continues on the server.
var errStillRunning = errors.New("stopped watching; the deployment carries on in Sprout, so check the app in the dashboard")

// deploy packs a folder, checks it against the server's own rules, uploads it,
// and watches the build until the app is live or the build fails. Nothing is
// uploaded when the check already shows the server would refuse it.
func deploy(ctx context.Context, args []string, env environment) error {
	folder, options, err := parseDeployArgs(args)
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(folder)
	if err != nil {
		return err
	}
	if info, err := os.Stat(absolute); err != nil || !info.IsDir() {
		return usageError{fmt.Sprintf("%s is not a folder", folder)}
	}
	slug := options.app
	if slug == "" {
		if slug = slugify(filepath.Base(absolute)); slug == "" {
			return usageError{"cannot make an app name from this folder's name; pass --app my-app"}
		}
	} else if !slugPattern.MatchString(slug) || len(slug) > 63 {
		return usageError{"--app must be lowercase letters, numbers and single dashes, like my-app"}
	}
	current, err := resolveSession(env.getenv, env.configDir)
	if err != nil {
		return err
	}
	client, err := apiclient.New(current.apiURL, current.token, userAgent)
	if err != nil {
		return err
	}

	// 1. Pack and check on this machine, before anything is sent.
	fmt.Fprintf(env.stdout, "Packing %s\n", absolute)
	archive, err := sourcepack.Pack(ctx, absolute)
	if err != nil {
		return err
	}
	fmt.Fprintf(env.stdout, "  %s, %d files, %s\n", describeKind(archive.Kind), archive.Files, formatSize(archive.Bytes))
	if len(archive.Skipped) > 0 {
		fmt.Fprintf(env.stdout, "  left out (never uploaded): %s\n", skippedList(archive.Skipped))
	}
	if err := sourcepack.Check(ctx, archive); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		message := "not uploaded, because Sprout would refuse it: " + explain(err.Error())
		if archive.BuiltFolder != "" {
			message += fmt.Sprintf("\nIf the finished website is in %q, deploy that instead: sprout deploy %s", archive.BuiltFolder, filepath.Join(folder, archive.BuiltFolder))
		}
		return errors.New(message)
	}

	// 2. Find (or create) the app on the server.
	workspace, application, err := findApplication(ctx, client, options.workspace, slug, filepath.Base(absolute), env)
	if err != nil {
		return err
	}

	// 3. Upload.
	fmt.Fprintf(env.stdout, "Uploading to %s/%s\n", workspace, application.Slug)
	uploadCtx, cancel := context.WithTimeout(ctx, uploadTimeout)
	created, err := client.CreateDeployment(uploadCtx, workspace, application.ID, archive.Tar)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return interrupted{"interrupted during the upload; if it finished, the deployment carries on in Sprout"}
		}
		return describeUploadError(err)
	}

	// 4. Watch until it ends.
	started := time.Now()
	final, err := watch(ctx, client, workspace, application.ID, created, env)
	if err != nil {
		return err
	}
	return report(ctx, client, workspace, application, final, time.Since(started), env)
}

type deployOptions struct{ app, workspace string }

// parseDeployArgs accepts the folder before or after the flags.
func parseDeployArgs(args []string) (string, deployOptions, error) {
	var options deployOptions
	flags := flag.NewFlagSet("deploy", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.app, "app", "", "app slug")
	flags.StringVar(&options.workspace, "workspace", "", "workspace slug")
	if err := flags.Parse(args); err != nil {
		return "", options, usageError{deployUsage}
	}
	folder := "."
	if rest := flags.Args(); len(rest) > 0 {
		folder = rest[0]
		if err := flags.Parse(rest[1:]); err != nil || flags.NArg() != 0 {
			return "", options, usageError{deployUsage}
		}
	}
	return folder, options, nil
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
func findApplication(ctx context.Context, client *apiclient.Client, workspace, slug, name string, env environment) (string, apiclient.Application, error) {
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
		return "", apiclient.Application{}, fmt.Errorf("an app called %q already exists in %s but you cannot see it; choose another name with --app", slug, workspace)
	}
	if err != nil {
		return "", apiclient.Application{}, err
	}
	fmt.Fprintf(env.stdout, "Created the app %q in %s\n", slug, workspace)
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

// watch polls the deployment and prints each stage as it finishes.
func watch(ctx context.Context, client *apiclient.Client, workspace, applicationID string, deployment apiclient.Deployment, env environment) (apiclient.Deployment, error) {
	deadline := time.Now().Add(watchLimit)
	printed := map[string]bool{}
	announcedQueue := false
	failures := 0
	for {
		if deployment.Status == "queued" && !announcedQueue {
			fmt.Fprintln(env.stdout, "  waiting for a build slot")
			announcedQueue = true
		}
		for _, stage := range deployment.Stages {
			if printed[stage.Name] {
				continue
			}
			switch stage.Status {
			case "succeeded":
				fmt.Fprintf(env.stdout, "  ✓ %s\n", stage.Name)
			case "failed":
				fmt.Fprintf(env.stdout, "  ✗ %s\n", stage.Name)
			default:
				continue
			}
			printed[stage.Name] = true
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

// report prints how the deployment ended and returns an error unless it worked.
func report(ctx context.Context, client *apiclient.Client, workspace string, application apiclient.Application, deployment apiclient.Deployment, took time.Duration, env environment) error {
	switch deployment.Status {
	case "live":
		ctx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()
		latest, err := client.GetApplication(ctx, workspace, application.ID)
		if err == nil && latest.URL != nil {
			fmt.Fprintf(env.stdout, "Live in %s: %s\n", took.Round(time.Second), *latest.URL)
		} else {
			fmt.Fprintf(env.stdout, "Live in %s. Open the app in the dashboard for its address.\n", took.Round(time.Second))
		}
		return nil
	case "succeeded":
		fmt.Fprintln(env.stdout, "Built and stored, but this server does not run apps, so there is no address yet.")
		return nil
	case "cancelled":
		return errors.New("the deployment was cancelled")
	}
	return errors.New("the deployment failed: " + explain(failureCode(deployment)))
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

func describeKind(kind sourcepack.Kind) string {
	if kind == sourcepack.KindNode {
		return "Node.js app"
	}
	return "website"
}

func skippedList(skipped []sourcepack.Skip) string {
	const max = 6
	names := make([]string, 0, max)
	for _, skip := range skipped[:min(len(skipped), max)] {
		names = append(names, skip.Path)
	}
	list := strings.Join(names, ", ")
	if len(skipped) > max {
		list += fmt.Sprintf(" and %d more", len(skipped)-max)
	}
	return list
}

func formatSize(bytes int64) string {
	if bytes >= 1<<20 {
		return fmt.Sprintf("%.1f MiB", float64(bytes)/(1<<20))
	}
	return fmt.Sprintf("%d KiB", max(1, (bytes+512)>>10))
}
