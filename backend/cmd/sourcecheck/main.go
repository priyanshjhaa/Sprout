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
	if flags.Parse(args) != nil || *archive == "" || flags.NArg() != 0 || (*runtime != "" && *runtime != "node") {
		fmt.Fprintln(diagnostics, "usage: sourcecheck -archive <local-source.tar> [-runtime node]")
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
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	summary, err := source.WithArchive(ctx, file, func(ctx context.Context, files fs.FS) error {
		if *runtime == "node" {
			return nodeapp.Validate(ctx, files)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 1
	}
	if json.NewEncoder(out).Encode(summary) != nil {
		return 1
	}
	return 0
}
