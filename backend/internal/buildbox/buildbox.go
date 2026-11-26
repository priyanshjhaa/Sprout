// Package buildbox runs one build script in a sealed, disposable Docker
// container: no network, no capabilities, a read-only root, an unprivileged
// user and bounded CPU, memory, processes and output. The input arrives as a
// tar on stdin and the output leaves as a tar on stdout. Each language's
// builder (nodeapp, pythonapp) supplies the image, the script and what the
// script's exit codes mean; the container's security posture lives only here.
package buildbox

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os/exec"
	"path"
	"time"
)

var (
	// ErrDocker means Docker could not run the container at all.
	ErrDocker = errors.New("build docker unavailable")
	// ErrTooLarge means the output exceeded its limit.
	ErrTooLarge = errors.New("build output too large")
	// ErrCleanup means the container could not be removed afterwards.
	ErrCleanup = errors.New("build cleanup failed")
	// ErrFiles means a file could not be read or written while packing.
	ErrFiles = errors.New("build files unreadable")
	// ErrLimit means packed files exceeded their limit.
	ErrLimit = errors.New("build files over limit")
)

// ExitError is the script's own non-zero exit, which the builder interprets.
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return "build script failed" }

// Box is one kind of build container.
type Box struct {
	Image   string
	Purpose string   // the io.sprout.purpose label, such as local-node-build
	Env     []string // extra NAME=value settings for the script
}

// Run executes script with input on stdin and returns at most limit bytes of
// stdout. ctx bounds the run; the container is removed before Run returns.
func (b Box) Run(ctx context.Context, input io.Reader, script string, limit int) (output []byte, err error) {
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return nil, ErrDocker
	}
	name := "sprout-local-build-" + hex.EncodeToString(random[:])
	commandCtx, stop := context.WithCancel(ctx)
	defer stop()
	bounded := &boundedOutput{limit: limit, cancel: stop}
	command := exec.CommandContext(commandCtx, "docker", b.args(name, script)...)
	command.Stdin = input
	command.Stdout = bounded
	command.Stderr = io.Discard
	command.WaitDelay = 5 * time.Second

	// Cleanup has its own bounded context so cancellation still stops the
	// specific container after the local Docker CLI has been interrupted.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if cleanupContainer(cleanupCtx, name, commandCtx.Err() != nil) != nil {
			err = errors.Join(err, ErrCleanup)
		}
	}()

	runErr := command.Run()
	if bounded.exceeded {
		return nil, ErrTooLarge
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if runErr == nil {
		return bounded.buffer.Bytes(), nil
	}
	var exit *exec.ExitError
	if errors.As(runErr, &exit) {
		switch code := exit.ExitCode(); code {
		case 125, 126, 127: // Docker itself could not run the container
		default:
			return nil, &ExitError{Code: code}
		}
	}
	return nil, ErrDocker
}

// args is the build container's whole security posture. /workspace is
// mounted exec (Docker's default for tmpfs is noexec) because builds run
// package commands such as vite and native helpers such as esbuild; that
// grants nothing new, since the build can already run any code through its
// language's interpreter.
func (b Box) args(name, script string) []string {
	args := []string{
		"run", "--name", name, "--pull=never", "--interactive", "--log-driver=none",
		"--label", "io.sprout.purpose=" + b.Purpose, "--label", "io.sprout.owned=" + name,
		"--network", "none", "--user", "65532:65532", "--read-only", "--cap-drop", "ALL",
		"--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2g", "--memory-swap", "2g", "--pids-limit", "256",
		"--tmpfs", "/workspace:rw,exec,nosuid,nodev,size=1073741824,uid=65532,gid=65532,mode=0700",
		"--tmpfs", "/tmp:rw,nosuid,nodev,size=536870912,uid=65532,gid=65532,mode=0700",
		"--workdir", "/workspace", "--env", "HOME=/tmp",
		"--env", "HTTP_PROXY=", "--env", "HTTPS_PROXY=", "--env", "ALL_PROXY=", "--env", "NO_PROXY=",
		"--env", "http_proxy=", "--env", "https_proxy=", "--env", "all_proxy=", "--env", "no_proxy=",
	}
	for _, setting := range b.Env {
		args = append(args, "--env", setting)
	}
	return append(args, b.Image, "sh", "-c", script)
}

type boundedOutput struct {
	buffer   bytes.Buffer
	limit    int
	cancel   context.CancelFunc
	exceeded bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.exceeded {
		return len(p), nil
	}
	remaining := b.limit - b.buffer.Len()
	if len(p) > remaining {
		if remaining > 0 {
			_, _ = b.buffer.Write(p[:remaining])
		}
		b.exceeded = true
		b.cancel()
		return len(p), nil
	}
	_, _ = b.buffer.Write(p)
	return len(p), nil
}

func cleanupContainer(ctx context.Context, name string, waitForLateCreate bool) error {
	for {
		inspect := exec.CommandContext(ctx, "docker", "container", "inspect", name)
		inspect.Stdout = io.Discard
		inspect.Stderr = io.Discard
		if inspect.Run() == nil {
			remove := exec.CommandContext(ctx, "docker", "container", "rm", "--force", "--", name)
			remove.Stdout = io.Discard
			remove.Stderr = io.Discard
			if remove.Run() == nil {
				return nil
			}
		} else if !waitForLateCreate {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// Pack writes files into writer under prefix with platform-owned metadata:
// fixed modes, no owners, no times. When limit is positive, size (the bytes
// written so far) must stay within it. With no prefix the archive is
// finished; with one, the caller adds more entries and closes it.
func Pack(ctx context.Context, files fs.FS, prefix string, writer *tar.Writer, size *bytes.Buffer, limit int64) error {
	err := fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return ErrFiles
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		target := name
		if prefix != "" {
			target = path.Join(prefix, name)
		}
		if entry.IsDir() {
			return writer.WriteHeader(&tar.Header{Name: target + "/", Mode: 0700, Typeflag: tar.TypeDir})
		}
		content, err := fs.ReadFile(files, name)
		if err != nil {
			return ErrFiles
		}
		if err := writer.WriteHeader(&tar.Header{Name: target, Mode: 0600, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			return ErrFiles
		}
		if _, err := writer.Write(content); err != nil {
			return ErrFiles
		}
		if limit > 0 && int64(size.Len()) > limit {
			return ErrLimit
		}
		return nil
	})
	if err != nil {
		return err
	}
	if prefix == "" {
		if writer.Close() != nil {
			return ErrFiles
		}
		if limit > 0 && int64(size.Len()) > limit {
			return ErrLimit
		}
	}
	return nil
}
