// sprout is the command-line front door to the Sprout deploy API. It is a
// client of the same public API as the dashboard and authenticates with a
// personal access token.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/apiclient"
)

const userAgent = "sprout-cli/dev"

// requestTimeout bounds a single short API call.
const requestTimeout = 15 * time.Second

const usage = `Sprout deploys small apps from a folder.

Usage:
  sprout login [--api URL]   store an access token for this machine
  sprout logout              forget the stored token
  sprout whoami              show who the token belongs to

Environment:
  SPROUT_TOKEN     access token to use instead of the stored one
  SPROUT_API_URL   API address (default ` + defaultAPIURL + `)
`

// environment is everything a command touches outside its arguments, so tests
// can run commands without a terminal, a home directory or real variables.
type environment struct {
	stdout, stderr io.Writer
	getenv         func(string) string
	configDir      string
	// readSecret reads a token without echoing it when stdin is a terminal.
	readSecret func(prompt string) (string, error)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	env := environment{stdout: os.Stdout, stderr: os.Stderr, getenv: os.Getenv}
	env.readSecret = terminalSecretReader(os.Stdin, os.Stderr)
	dir, err := configDir(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sprout: cannot find a home directory for the config:", err)
		os.Exit(1)
	}
	env.configDir = dir
	os.Exit(run(ctx, os.Args[1:], env))
}

// Exit codes: 0 success, 1 the operation failed, 2 usage or authentication.
func run(ctx context.Context, args []string, env environment) int {
	if len(args) == 0 {
		fmt.Fprint(env.stderr, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "login":
		err = login(ctx, args[1:], env)
	case "logout":
		err = logout(args[1:], env)
	case "whoami":
		err = whoami(ctx, args[1:], env)
	case "help", "-h", "--help":
		fmt.Fprint(env.stdout, usage)
		return 0
	default:
		fmt.Fprintf(env.stderr, "sprout: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
	if err == nil {
		return 0
	}
	fmt.Fprintln(env.stderr, "sprout:", describe(err))
	return exitCode(err)
}

// usageError marks a mistake in how the command was called.
type usageError struct{ message string }

func (e usageError) Error() string { return e.message }

func exitCode(err error) int {
	var usage usageError
	if errors.As(err, &usage) || errors.Is(err, errNotLoggedIn) || errors.Is(err, errTokenShape) || errors.Is(err, apiclient.ErrInvalidBaseURL) ||
		apiclient.HasCode(err, "authentication_required") || apiclient.HasCode(err, "token_not_permitted") {
		return 2
	}
	return 1
}

// describe turns an error into one line for a person. API errors keep their
// stable code so they can be searched for and reported.
func describe(err error) string {
	switch {
	case apiclient.HasCode(err, "authentication_required"):
		return "the access token was refused (invalid, expired or revoked); create a new one in the dashboard and run `sprout login`"
	case errors.Is(err, context.DeadlineExceeded):
		return "the API did not answer in time"
	case errors.Is(err, context.Canceled):
		return "interrupted"
	}
	var apiErr *apiclient.Error
	if errors.As(err, &apiErr) {
		return apiErr.Error()
	}
	var network interface{ Timeout() bool }
	if errors.As(err, &network) {
		return "cannot reach the Sprout API: " + err.Error()
	}
	return err.Error()
}
