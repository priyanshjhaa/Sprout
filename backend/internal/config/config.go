package config

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	DefaultAPIAddress = "127.0.0.1:8080"
	DefaultWebOrigin  = "http://localhost:3000"

	DefaultAppsAddress = "127.0.0.1:8081"
	DefaultAppsDomain  = "localhost"
)

type Config struct {
	APIAddress  string
	DatabaseURL string
	// WebOrigins are the browser origins allowed to call the API: CORS answers
	// them, and Clerk sessions must have been issued to one of them.
	WebOrigins     []string
	ClerkSecretKey string
	// LocalBuilds enables accepting source uploads and running real builds in
	// Docker on this machine. It is local developer tooling, not a service.
	LocalBuilds LocalBuilds
	// Apps configures the public listener that serves live applications. It is
	// only started when local builds are enabled.
	Apps Apps
}

// Apps is the address and base domain of the application proxy. Applications
// are served at {application}.{workspace}.{Domain}. The address must be
// loopback: this is local developer infrastructure, not an internet-facing edge.
type Apps struct {
	Address string
	Domain  string
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

	webOrigins, err := loadWebOrigins(lookupEnv)
	if err != nil {
		return Config{}, err
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
	apps, err := loadApps(lookupEnv)
	if err != nil {
		return Config{}, err
	}

	return Config{
		APIAddress:     address,
		DatabaseURL:    databaseURL,
		WebOrigins:     webOrigins,
		ClerkSecretKey: clerkSecretKey,
		LocalBuilds:    builds,
		Apps:           apps,
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

var domainLabel = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func loadApps(lookupEnv LookupEnv) (Apps, error) {
	apps := Apps{Address: DefaultAppsAddress, Domain: DefaultAppsDomain}
	if value, exists := lookupEnv("SPROUT_APPS_ADDRESS"); exists {
		apps.Address = strings.TrimSpace(value)
	}
	if err := validateAddress(apps.Address); err != nil {
		return Apps{}, fmt.Errorf("SPROUT_APPS_ADDRESS: %w", err)
	}
	host, _, _ := net.SplitHostPort(apps.Address)
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return Apps{}, fmt.Errorf("SPROUT_APPS_ADDRESS must be on a loopback host")
	}
	if value, exists := lookupEnv("SPROUT_APPS_DOMAIN"); exists {
		apps.Domain = strings.ToLower(strings.Trim(strings.TrimSpace(value), "."))
	}
	for _, label := range strings.Split(apps.Domain, ".") {
		if len(label) > 63 || !domainLabel.MatchString(label) {
			return Apps{}, fmt.Errorf("SPROUT_APPS_DOMAIN must be a valid DNS name such as localhost")
		}
	}
	return apps, nil
}

// URL is where a live application is reachable from this machine.
func (a Apps) URL(workspaceSlug, applicationSlug string) string {
	url := "http://" + applicationSlug + "." + workspaceSlug + "." + a.Domain
	if _, port, err := net.SplitHostPort(a.Address); err == nil && port != "80" {
		url += ":" + port
	}
	return url
}

// loadWebOrigins reads SPROUT_WEB_ORIGIN as one origin or a comma-separated
// list, so a second frontend (another branch on another port) can run beside
// the first. Every entry must be an exact origin; there is no wildcard.
func loadWebOrigins(lookupEnv func(string) (string, bool)) ([]string, error) {
	value, exists := lookupEnv("SPROUT_WEB_ORIGIN")
	if !exists {
		return []string{DefaultWebOrigin}, nil
	}
	var origins []string
	for _, entry := range strings.Split(value, ",") {
		origin := strings.TrimSpace(entry)
		if !validWebOrigin(origin) {
			return nil, fmt.Errorf("SPROUT_WEB_ORIGIN must be one or more valid HTTP origins without a path, separated by commas")
		}
		if !slices.Contains(origins, origin) {
			origins = append(origins, origin)
		}
	}
	return origins, nil
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
