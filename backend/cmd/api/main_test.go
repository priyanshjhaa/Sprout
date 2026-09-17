package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/priyanshjhaa/Sprout/backend/internal/config"
	"github.com/priyanshjhaa/Sprout/backend/internal/httpapi"
)

func TestRunReportsListenFailure(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router := httpapi.NewRouter(logger, httpapi.AlwaysReady)

	exitCode := run(
		context.Background(),
		config.Config{APIAddress: "127.0.0.1:not-a-port"},
		logger,
		router,
	)

	if exitCode != 1 {
		t.Fatalf("run() exit code = %d, want 1", exitCode)
	}

	if !strings.Contains(logs.String(), `"msg":"listen failed"`) {
		t.Fatalf("run() logs = %q, want listen failure", logs.String())
	}
}
