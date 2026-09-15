package config

import (
	"strings"
	"testing"
)

func TestLoadUsesDefaultAddress(t *testing.T) {
	t.Parallel()

	config, err := Load(func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if config.APIAddress != DefaultAPIAddress {
		t.Fatalf("APIAddress = %q, want %q", config.APIAddress, DefaultAPIAddress)
	}
}

func TestLoadUsesConfiguredAddress(t *testing.T) {
	t.Parallel()

	config, err := Load(func(key string) (string, bool) {
		if key == "SPROUT_API_ADDRESS" {
			return " 0.0.0.0:9090 ", true
		}
		return "", false
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if config.APIAddress != "0.0.0.0:9090" {
		t.Fatalf("APIAddress = %q, want 0.0.0.0:9090", config.APIAddress)
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

			_, err := Load(func(string) (string, bool) { return test.address, true })
			if err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), "SPROUT_API_ADDRESS") {
				t.Fatalf("Load() error = %q, want variable name", err)
			}
		})
	}
}
