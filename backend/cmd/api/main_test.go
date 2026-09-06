package main

import (
	"bytes"
	"testing"
)

func TestRunReportsReadyAndSucceeds(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer

	exitCode := run(&stdout)

	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, want 0", exitCode)
	}

	const expected = "sprout-api: backend foundation ready\n"
	if stdout.String() != expected {
		t.Fatalf("run() output = %q, want %q", stdout.String(), expected)
	}
}
