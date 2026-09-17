package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const (
	DefaultAPIAddress = "127.0.0.1:8080"
	DefaultWebOrigin  = "http://localhost:3000"
)

type Config struct {
	APIAddress  string
	DatabaseURL string
	WebOrigin   string
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

	return Config{
		APIAddress:  address,
		DatabaseURL: databaseURL,
		WebOrigin:   webOrigin,
	}, nil
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
