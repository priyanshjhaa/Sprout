// sourcecheck is a local developer tool, not a public upload or deployment API.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
	"github.com/priyanshjhaa/Sprout/backend/internal/nodeapp"
	"github.com/priyanshjhaa/Sprout/backend/internal/source"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, out, diagnostics io.Writer) int {
	flags := flag.NewFlagSet("sourcecheck", flag.ContinueOnError)
	flags.SetOutput(io.Discard) // Input paths and arbitrary arguments may be sensitive.
	archive := flags.String("archive", "", "plain uncompressed source tar archive")
	runtime := flags.String("runtime", "", "optional application contract: node")
	buildNode := flags.Bool("build-node", false, "run the Node build scripts in the restricted local Docker smoke runner")
	artifactDirectory := flags.String("artifact-dir", "", "existing private directory for the local build artifact")
	if flags.Parse(args) != nil || *archive == "" || flags.NArg() != 0 || (*runtime != "" && *runtime != "node") || (*buildNode && *runtime == "node") || (*artifactDirectory != "" && !*buildNode) {
		fmt.Fprintln(diagnostics, "usage: sourcecheck -archive <local-source.tar> [-runtime node | -build-node [-artifact-dir <artifact-directory>]]")
		return 2
	}
	info, err := os.Lstat(*archive)
	if err != nil || !info.Mode().IsRegular() || info.Size() > source.MaxArchiveBytes {
		fmt.Fprintln(diagnostics, "source_input_invalid")
		return 1
	}
	file, err := os.Open(*archive)
	if err != nil {
		fmt.Fprintln(diagnostics, "source_input_unavailable")
		return 1
	}
	defer file.Close()
	// Recheck the opened object before reading; reject symlinks and special inputs.
	actual, err := file.Stat()
	if err != nil || !actual.Mode().IsRegular() || !os.SameFile(info, actual) {
		fmt.Fprintln(diagnostics, "source_input_invalid")
		return 1
	}
	var store *artifact.Store
	if *artifactDirectory != "" {
		store, err = artifact.Open(*artifactDirectory)
		if err != nil {
			fmt.Fprintln(diagnostics, err)
			return 1
		}
		defer store.Close()
	}
	timeout := 10 * time.Second
	if *buildNode {
		timeout = 2*time.Minute + 15*time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var built []byte
	summary, err := source.WithArchive(ctx, file, func(ctx context.Context, files fs.FS) error {
		if *runtime == "node" {
			return nodeapp.Validate(ctx, files)
		}
		if *buildNode {
			runner, err := nodeapp.NewRunner(nodeapp.DefaultImage, 2*time.Minute)
			if err != nil {
				return err
			}
			if store == nil {
				return runner.RunFiles(ctx, files)
			}
			built, err = runner.BuildFiles(ctx, files)
			return err
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 1
	}
	var saved *artifact.Descriptor
	if store != nil {
		descriptor, err := store.Save(ctx, built)
		if err != nil {
			fmt.Fprintln(diagnostics, err)
			return 1
		}
		saved = &descriptor
	}
	response := struct {
		source.Summary
		Artifact *artifact.Descriptor `json:"artifact,omitempty"`
	}{Summary: summary, Artifact: saved}
	if json.NewEncoder(out).Encode(response) != nil {
		return 1
	}
	return 0
}
