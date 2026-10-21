package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/priyanshjhaa/Sprout/backend/internal/apiclient"
)

const (
	defaultAPIURL   = "http://127.0.0.1:8080"
	credentialsFile = "credentials.json"
	// tokenPrefix mirrors apitoken.Prefix; a test keeps them equal. Importing
	// apitoken would link the database driver into the CLI.
	tokenPrefix = "sprout_pat_"
)

var (
	errNotLoggedIn = errors.New("not logged in: run `sprout login`, or set SPROUT_TOKEN")
	errTokenShape  = errors.New("that is not a Sprout access token (they start with " + tokenPrefix + "); create one on the dashboard's Access tokens page")
)

// credentials is what `sprout login` stores. The token is kept with the API
// address it was verified against, and is only ever sent back to that address.
type credentials struct {
	APIURL string `json:"apiUrl"`
	Token  string `json:"token"`
}

// session is the resolved API address and token for one command.
type session struct {
	apiURL, token, tokenSource string
}

// configDir is $XDG_CONFIG_HOME/sprout, or ~/.config/sprout.
func configDir(getenv func(string) string) (string, error) {
	if base := getenv("XDG_CONFIG_HOME"); base != "" && filepath.IsAbs(base) {
		return filepath.Join(base, "sprout"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "sprout"), nil
}

// apiURL picks the API address: SPROUT_API_URL, then the stored login, then the
// local default.
func apiURL(getenv func(string) string, stored *credentials) (string, error) {
	raw := getenv("SPROUT_API_URL")
	if raw == "" && stored != nil {
		raw = stored.APIURL
	}
	if raw == "" {
		raw = defaultAPIURL
	}
	return apiclient.NormalizeBaseURL(raw)
}

// resolveSession finds the token for a command. SPROUT_TOKEN wins, for CI and
// agents; otherwise the stored token is used, but only for the address it was
// stored for.
func resolveSession(getenv func(string) string, dir string) (session, error) {
	stored, err := readCredentials(dir)
	if err != nil {
		return session{}, err
	}
	address, err := apiURL(getenv, stored)
	if err != nil {
		return session{}, err
	}
	if token := strings.TrimSpace(getenv("SPROUT_TOKEN")); token != "" {
		if !strings.HasPrefix(token, tokenPrefix) {
			return session{}, fmt.Errorf("SPROUT_TOKEN: %w", errTokenShape)
		}
		return session{apiURL: address, token: token, tokenSource: "SPROUT_TOKEN"}, nil
	}
	if stored == nil {
		return session{}, errNotLoggedIn
	}
	if stored.APIURL != address {
		return session{}, fmt.Errorf("you are logged in to %s, not %s: run `sprout login` for this address", stored.APIURL, address)
	}
	return session{apiURL: address, token: stored.Token, tokenSource: filepath.Join(dir, credentialsFile)}, nil
}

// readCredentials returns nil when nobody has logged in. A file that others can
// read is refused rather than used, the way ssh treats a private key.
func readCredentials(dir string) (*credentials, error) {
	path := filepath.Join(dir, credentialsFile)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s can be read by other users; run `chmod 600 %s` or `sprout login` again", path, path)
	}
	var stored credentials
	if err := json.NewDecoder(io.LimitReader(file, 64<<10)).Decode(&stored); err != nil || stored.APIURL == "" || !strings.HasPrefix(stored.Token, tokenPrefix) {
		return nil, fmt.Errorf("%s is damaged; run `sprout login` again", path)
	}
	return &stored, nil
}

// writeCredentials replaces the file atomically, private from the first byte:
// os.CreateTemp creates files with mode 0600.
func writeCredentials(dir string, stored credentials) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".credentials-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if err := json.NewEncoder(temporary).Encode(stored); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), filepath.Join(dir, credentialsFile))
}

// removeCredentials reports whether there was a stored login to remove.
func removeCredentials(dir string) (bool, error) {
	err := os.Remove(filepath.Join(dir, credentialsFile))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
