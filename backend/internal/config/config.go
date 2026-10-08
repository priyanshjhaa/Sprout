package config

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultAPIAddress = "127.0.0.1:8080"
	DefaultWebOrigin  = "http://localhost:3000"
)

type Config struct {
	APIAddress     string
	DatabaseURL    string
	WebOrigin      string
	ClerkSecretKey string
	// LocalBuilds enables accepting source uploads and running real builds in
	// Docker on this machine. It is local developer tooling, not a service.
	LocalBuilds LocalBuilds
}

// LocalBuilds is enabled only with SPROUT_ENABLE_LOCAL_BUILDS=1, an API bound to
// a loopback address, and two existing private directories for source uploads
// and build artifacts.
type LocalBuilds struct {
	Enabled           bool
	SourceDirectory   string
	ArtifactDirectory string
}

type LookupEnv func(key string) (string, bool)

func Load(lookupEnv LookupEnv) (Config, error) {
	address := DefaultAPIAddress
	if value, exists := lookupEnv("SPROUT_API_ADDRESS"); exists {
		address = strings.TrimSpace(value)
	}

	if err := validateAddress(address); err != nil {
		return Config{}, fmt.Errorf("SPROUT_API_ADDRESS: %w", err)
	}

	databaseURL, exists := lookupEnv("DATABASE_URL")
	databaseURL = strings.TrimSpace(databaseURL)
	if !exists || databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if !validDatabaseURL(databaseURL) {
		return Config{}, fmt.Errorf("DATABASE_URL must be a valid PostgreSQL connection URL")
	}

	webOrigin := DefaultWebOrigin
	if value, exists := lookupEnv("SPROUT_WEB_ORIGIN"); exists {
		webOrigin = strings.TrimSpace(value)
	}
	if !validWebOrigin(webOrigin) {
		return Config{}, fmt.Errorf("SPROUT_WEB_ORIGIN must be a valid HTTP origin without a path")
	}
	clerkSecretKey, exists := lookupEnv("CLERK_SECRET_KEY")
	clerkSecretKey = strings.TrimSpace(clerkSecretKey)
	if !exists || clerkSecretKey == "" {
		return Config{}, fmt.Errorf("CLERK_SECRET_KEY is required")
	}

	builds, err := loadLocalBuilds(lookupEnv, address)
	if err != nil {
		return Config{}, err
	}

	return Config{
		APIAddress:     address,
		DatabaseURL:    databaseURL,
		WebOrigin:      webOrigin,
		ClerkSecretKey: clerkSecretKey,
		LocalBuilds:    builds,
	}, nil
}

func loadLocalBuilds(lookupEnv LookupEnv, address string) (LocalBuilds, error) {
	value, _ := lookupEnv("SPROUT_ENABLE_LOCAL_BUILDS")
	switch strings.TrimSpace(value) {
	case "", "0":
		return LocalBuilds{}, nil
	case "1":
	default:
		return LocalBuilds{}, fmt.Errorf("SPROUT_ENABLE_LOCAL_BUILDS must be 1 or unset")
	}
	// Builds run untrusted code with Docker on this machine; never offer that to
	// anything but this machine.
	host, _, _ := net.SplitHostPort(address)
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return LocalBuilds{}, fmt.Errorf("SPROUT_ENABLE_LOCAL_BUILDS requires SPROUT_API_ADDRESS on a loopback host")
	}
	builds := LocalBuilds{Enabled: true}
	for _, setting := range []struct {
		name   string
		target *string
	}{{"SPROUT_SOURCE_DIR", &builds.SourceDirectory}, {"SPROUT_ARTIFACT_DIR", &builds.ArtifactDirectory}} {
		directory, _ := lookupEnv(setting.name)
		directory = strings.TrimSpace(directory)
		if !filepath.IsAbs(directory) {
			return LocalBuilds{}, fmt.Errorf("%s must be an absolute path when local builds are enabled", setting.name)
		}
		*setting.target = filepath.Clean(directory)
	}
	if builds.SourceDirectory == builds.ArtifactDirectory {
		return LocalBuilds{}, fmt.Errorf("SPROUT_SOURCE_DIR and SPROUT_ARTIFACT_DIR must be different directories")
	}
	return builds, nil
}

func validWebOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	return (parsed.Scheme == "http" || parsed.Scheme == "https") &&
		parsed.Host != "" && parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validDatabaseURL(databaseURL string) bool {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return false
	}

	validScheme := parsed.Scheme == "postgres" || parsed.Scheme == "postgresql"
	databaseName := strings.Trim(parsed.Path, "/")

	return validScheme && parsed.Host != "" && parsed.User != nil && databaseName != ""
}

func validateAddress(address string) error {
	if address == "" {
		return fmt.Errorf("must not be empty")
	}

	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("must use host:port format: %w", err)
	}

	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	return nil
}
