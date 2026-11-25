package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/apiclient"
	"github.com/priyanshjhaa/Sprout/backend/internal/sourcepack"
)

const deployUsage = "usage: sprout deploy [folder] [--app slug] [--workspace slug]"

// deploy is `sprout deploy`: the shared deploy flow (deployflow.go), printed
// line by line to the terminal.
func deploy(ctx context.Context, args []string, env environment) error {
	folder, options, err := parseDeployArgs(args)
	if err != nil {
		return err
	}
	request, err := newDeployRequest(folder, options.app, options.workspace)
	if err != nil {
		return err
	}
	current, err := resolveSession(env.getenv, env.configDir)
	if err != nil {
		return err
	}
	client, err := apiclient.New(current.apiURL, current.token, userAgent)
	if err != nil {
		return err
	}
	result, err := deployFolder(ctx, client, request, printer{env.stdout})
	var refused refusedError
	if errors.As(err, &refused) && refused.builtFolder != "" {
		return fmt.Errorf("%w\nIf the finished website is in %q, deploy that instead: sprout deploy %s", err, refused.builtFolder, filepath.Join(folder, refused.builtFolder))
	}
	if err != nil {
		return err
	}
	switch {
	case result.deployment.Status == "succeeded":
		fmt.Fprintln(env.stdout, "Built and stored, but this server does not run apps, so there is no address yet.")
	case result.url != "":
		fmt.Fprintf(env.stdout, "Live in %s: %s\n", result.took.Round(time.Second), result.url)
	default:
		fmt.Fprintf(env.stdout, "Live in %s. Open the app in the dashboard for its address.\n", result.took.Round(time.Second))
	}
	return nil
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

// printer is the terminal's deployObserver: one line per event.
type printer struct{ out io.Writer }

func (p printer) packing(folder string) { fmt.Fprintf(p.out, "Packing %s\n", folder) }

func (p printer) packed(archive sourcepack.Archive) {
	fmt.Fprintf(p.out, "  %s, %d files, %s\n", describeKind(archive.Kind), archive.Files, formatSize(archive.Bytes))
	if len(archive.Skipped) > 0 {
		fmt.Fprintf(p.out, "  left out (never uploaded): %s\n", skippedList(archive.Skipped))
	}
}

func (p printer) appCreated(workspace, slug string) {
	fmt.Fprintf(p.out, "Created the app %q in %s\n", slug, workspace)
}

func (p printer) uploading(workspace, slug string) {
	fmt.Fprintf(p.out, "Uploading to %s/%s\n", workspace, slug)
}

func (p printer) queued() { fmt.Fprintln(p.out, "  waiting for a build slot") }

func (p printer) stageFinished(stage string, succeeded bool) {
	mark := "✓"
	if !succeeded {
		mark = "✗"
	}
	fmt.Fprintf(p.out, "  %s %s\n", mark, stage)
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
