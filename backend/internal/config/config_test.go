package config

import (
	"strings"
	"testing"
)

const testDatabaseURL = "postgresql://sprout:local-password@127.0.0.1:5432/sprout"

func lookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		if key == "CLERK_SECRET_KEY" {
			value, exists := values[key]
			if exists {
				return value, true
			}
			return "sk_test_example", true
		}
		value, exists := values[key]
		return value, exists
	}
}

func TestLoadRequiresClerkSecretKey(t *testing.T) {
	t.Parallel()
	_, err := Load(lookup(map[string]string{
		"DATABASE_URL":     testDatabaseURL,
		"CLERK_SECRET_KEY": "",
	}))
	if err == nil || err.Error() != "CLERK_SECRET_KEY is required" {
		t.Fatalf("Load() error = %v, want required Clerk key error", err)
	}
}

func TestLoadUsesDefaultAddress(t *testing.T) {
	t.Parallel()

	appConfig, err := Load(lookup(map[string]string{"DATABASE_URL": testDatabaseURL}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if appConfig.APIAddress != DefaultAPIAddress {
		t.Fatalf("APIAddress = %q, want %q", appConfig.APIAddress, DefaultAPIAddress)
	}
	if appConfig.DatabaseURL != testDatabaseURL {
		t.Fatalf("DatabaseURL = %q, want configured URL", appConfig.DatabaseURL)
	}
	if appConfig.WebOrigin != DefaultWebOrigin {
		t.Fatalf("WebOrigin = %q, want %q", appConfig.WebOrigin, DefaultWebOrigin)
	}
}

func TestLoadUsesConfiguredWebOrigin(t *testing.T) {
	t.Parallel()

	appConfig, err := Load(lookup(map[string]string{
		"DATABASE_URL":      testDatabaseURL,
		"SPROUT_WEB_ORIGIN": " https://sprout.example ",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if appConfig.WebOrigin != "https://sprout.example" {
		t.Fatalf("WebOrigin = %q, want https://sprout.example", appConfig.WebOrigin)
	}
}

func TestLoadRejectsInvalidWebOrigin(t *testing.T) {
	t.Parallel()

	_, err := Load(lookup(map[string]string{
		"DATABASE_URL":      testDatabaseURL,
		"SPROUT_WEB_ORIGIN": "http://localhost:3000/dashboard",
	}))
	if err == nil || !strings.Contains(err.Error(), "SPROUT_WEB_ORIGIN") {
		t.Fatalf("Load() error = %v, want SPROUT_WEB_ORIGIN validation error", err)
	}
}

func TestLoadUsesConfiguredAddress(t *testing.T) {
	t.Parallel()

	appConfig, err := Load(lookup(map[string]string{
		"SPROUT_API_ADDRESS": " 0.0.0.0:9090 ",
		"DATABASE_URL":       testDatabaseURL,
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if appConfig.APIAddress != "0.0.0.0:9090" {
		t.Fatalf("APIAddress = %q, want 0.0.0.0:9090", appConfig.APIAddress)
	}
}

func TestLoadRejectsInvalidAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		address string
	}{
		{name: "empty", address: ""},
		{name: "missing port", address: "localhost"},
		{name: "invalid port", address: "localhost:not-a-port"},
		{name: "zero port", address: "localhost:0"},
		{name: "port too large", address: "localhost:65536"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Load(lookup(map[string]string{
				"SPROUT_API_ADDRESS": test.address,
				"DATABASE_URL":       testDatabaseURL,
			}))
			if err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), "SPROUT_API_ADDRESS") {
				t.Fatalf("Load() error = %q, want variable name", err)
			}
		})
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Parallel()

	_, err := Load(lookup(nil))
	if err == nil || err.Error() != "DATABASE_URL is required" {
		t.Fatalf("Load() error = %v, want required DATABASE_URL error", err)
	}
}

func TestLoadRejectsInvalidDatabaseURLWithoutExposingIt(t *testing.T) {
	t.Parallel()

	const invalidURL = "postgresql://sprout:sensitive-password@/missing-host"
	_, err := Load(lookup(map[string]string{"DATABASE_URL": invalidURL}))

	if err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
	if strings.Contains(err.Error(), "sensitive-password") {
		t.Fatalf("Load() error exposes password: %v", err)
	}
}

func TestLocalBuilds(t *testing.T) {
	t.Parallel()
	base := map[string]string{"DATABASE_URL": testDatabaseURL}
	with := func(extra map[string]string) map[string]string {
		values := map[string]string{}
		for key, value := range base {
			values[key] = value
		}
		for key, value := range extra {
			values[key] = value
		}
		return values
	}
	enabled := map[string]string{"SPROUT_ENABLE_LOCAL_BUILDS": "1", "SPROUT_SOURCE_DIR": "/private/tmp/sprout-sources", "SPROUT_ARTIFACT_DIR": "/private/tmp/sprout-artifacts"}

	off, err := Load(lookup(base))
	if err != nil || off.LocalBuilds.Enabled {
		t.Fatalf("builds enabled by default: %+v %v", off.LocalBuilds, err)
	}
	on, err := Load(lookup(with(enabled)))
	if err != nil || !on.LocalBuilds.Enabled || on.LocalBuilds.SourceDirectory != "/private/tmp/sprout-sources" {
		t.Fatalf("enabled builds: %+v %v", on.LocalBuilds, err)
	}
	for name, extra := range map[string]map[string]string{
		"ambiguous flag":    {"SPROUT_ENABLE_LOCAL_BUILDS": "true"},
		"public address":    with(map[string]string{"SPROUT_API_ADDRESS": "0.0.0.0:8080"}),
		"relative source":   {"SPROUT_SOURCE_DIR": "sources"},
		"missing artifacts": {"SPROUT_ARTIFACT_DIR": ""},
		"same directory":    {"SPROUT_ARTIFACT_DIR": "/private/tmp/sprout-sources"},
	} {
		values := with(enabled)
		for key, value := range extra {
			values[key] = value
		}
		if _, err := Load(lookup(values)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
