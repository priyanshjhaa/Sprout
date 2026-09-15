package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

const DefaultAPIAddress = "127.0.0.1:8080"

type Config struct {
	APIAddress string
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

	return Config{APIAddress: address}, nil
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
