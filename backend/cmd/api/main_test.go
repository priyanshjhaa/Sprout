package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunReportsListenFailure(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(context.Background(), "127.0.0.1:not-a-port", &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("run() exit code = %d, want 1", exitCode)
	}

	if stdout.Len() != 0 {
		t.Fatalf("run() stdout = %q, want empty", stdout.String())
	}

	if !strings.Contains(stderr.String(), "sprout-api: listen:") {
		t.Fatalf("run() stderr = %q, want listen error", stderr.String())
	}
}
