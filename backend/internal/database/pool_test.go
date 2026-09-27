package database

import (
	"testing"
	"time"
)

func TestParsePoolConfigSetsConnectionBounds(t *testing.T) {
	t.Parallel()

	poolConfig, err := parsePoolConfig("postgresql://sprout:local-password@127.0.0.1:5432/sprout")
	if err != nil {
		t.Fatalf("parsePoolConfig() error = %v", err)
	}

	if poolConfig.MaxConns != maxConnections {
		t.Fatalf("MaxConns = %d, want %d", poolConfig.MaxConns, maxConnections)
	}
	if poolConfig.MinConns != 0 {
		t.Fatalf("MinConns = %d, want 0", poolConfig.MinConns)
	}
	if poolConfig.MaxConnLifetime != 30*time.Minute {
		t.Fatalf("MaxConnLifetime = %s, want 30m", poolConfig.MaxConnLifetime)
	}
	if poolConfig.MaxConnIdleTime != 5*time.Minute {
		t.Fatalf("MaxConnIdleTime = %s, want 5m", poolConfig.MaxConnIdleTime)
	}
}
