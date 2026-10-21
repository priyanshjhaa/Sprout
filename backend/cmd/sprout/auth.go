package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/priyanshjhaa/Sprout/backend/internal/apiclient"
	"golang.org/x/term"
)

// login verifies a token against the API before storing it, so a typo or a
// revoked token is caught now rather than at the next deploy. The token is
// read from stdin and never accepted as an argument, which would leave it in
// shell history and in the process list.
func login(ctx context.Context, args []string, env environment) error {
	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	api := flags.String("api", "", "API address")
	if err := flags.Parse(args); err != nil {
		return usageError{"usage: sprout login [--api URL]"}
	}
	if flags.NArg() != 0 {
		return usageError{"sprout login reads the token from stdin, not from arguments (arguments end up in shell history); run `sprout login` and paste it"}
	}
	raw := *api
	if raw == "" {
		raw = env.getenv("SPROUT_API_URL")
	}
	if raw == "" {
		raw = defaultAPIURL
	}
	address, err := apiclient.NormalizeBaseURL(raw)
	if err != nil {
		return usageError{err.Error()}
	}
	token, err := env.readSecret("Paste an access token from the dashboard's Access tokens page: ")
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, tokenPrefix) {
		return errTokenShape
	}
	me, err := identify(ctx, address, token)
	if err != nil {
		return err
	}
	if err := writeCredentials(env.configDir, credentials{APIURL: address, Token: token}); err != nil {
		return fmt.Errorf("the token works but could not be saved: %w", err)
	}
	fmt.Fprintf(env.stdout, "Logged in to %s as %s (workspace %s).\n", address, me.Email, me.Workspace.Slug)
	return nil
}

// logout forgets the stored token. Revoking it needs the dashboard, because an
// access token cannot manage tokens.
func logout(args []string, env environment) error {
	if len(args) != 0 {
		return usageError{"usage: sprout logout"}
	}
	removed, err := removeCredentials(env.configDir)
	if err != nil {
		return err
	}
	if !removed {
		fmt.Fprintln(env.stdout, "Not logged in.")
		return nil
	}
	fmt.Fprintln(env.stdout, "Logged out. The token still works until it expires; revoke it on the dashboard's Access tokens page if you no longer need it.")
	return nil
}

func whoami(ctx context.Context, args []string, env environment) error {
	if len(args) != 0 {
		return usageError{"usage: sprout whoami"}
	}
	current, err := resolveSession(env.getenv, env.configDir)
	if err != nil {
		return err
	}
	me, err := identify(ctx, current.apiURL, current.token)
	if err != nil {
		return err
	}
	fmt.Fprintf(env.stdout, "%s <%s>\nworkspace  %s (%s)\napi        %s\ntoken      from %s\n",
		me.DisplayName, me.Email, me.Workspace.Name, me.Workspace.Slug, current.apiURL, current.tokenSource)
	return nil
}

func identify(ctx context.Context, address, token string) (apiclient.Me, error) {
	client, err := apiclient.New(address, token, userAgent)
	if err != nil {
		return apiclient.Me{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	return client.Me(ctx)
}

// terminalSecretReader prompts and reads without echo on a terminal; piped
// input (`sprout login < token.txt`, CI) is read as one line.
func terminalSecretReader(stdin *os.File, prompt io.Writer) func(string) (string, error) {
	return func(message string) (string, error) {
		if !term.IsTerminal(int(stdin.Fd())) {
			return readLine(stdin)
		}
		fmt.Fprint(prompt, message)
		secret, err := term.ReadPassword(int(stdin.Fd()))
		fmt.Fprintln(prompt)
		return string(secret), err
	}
}

func readLine(input io.Reader) (string, error) {
	line, err := bufio.NewReader(io.LimitReader(input, 4<<10)).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return line, nil
}
