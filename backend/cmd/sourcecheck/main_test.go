package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
)

func TestSourcecheck(t *testing.T) {
	for _, name := range []string{"index.html", "../private-name", ".env"} {
		t.Run(name, func(t *testing.T) {
			var archive bytes.Buffer
			writer := tar.NewWriter(&archive)
			if err := writer.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR, Size: 2}); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Write([]byte("hi")); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			input := filepath.Join(directory, "private-source-name.tar")
			if err := os.WriteFile(input, archive.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			var out, diagnostics bytes.Buffer
			code := run(context.Background(), []string{"-archive", input}, &out, &diagnostics)
			if name == "index.html" {
				if code != 0 || out.String() != "{\"files\":1,\"bytes\":2}\n" || diagnostics.Len() != 0 {
					t.Fatalf("code=%d out=%s diagnostics=%s", code, &out, &diagnostics)
				}
			} else if code != 1 || out.Len() != 0 || strings.Contains(diagnostics.String(), name) || strings.Contains(diagnostics.String(), input) {
				t.Fatalf("unsafe result: %d %s %s", code, &out, &diagnostics)
			}
		})
	}
}

func TestSourcecheckInvalidInput(t *testing.T) {
	for _, args := range [][]string{nil, {"-unexpected-private-value"}, {"-archive", "missing-private-file"}} {
		var out, diagnostics bytes.Buffer
		if run(context.Background(), args, &out, &diagnostics) == 0 {
			t.Fatal("accepted invalid input")
		}
		if strings.Contains(diagnostics.String(), "private") {
			t.Fatal("arguments leaked")
		}
	}
}

func TestSourcecheckNodeContract(t *testing.T) {
	for _, valid := range []bool{true, false} {
		var archive bytes.Buffer
		writer := tar.NewWriter(&archive)
		for _, name := range []string{"package.json", "package-lock.json", "build.mjs", "server.mjs"} {
			data, err := os.ReadFile(filepath.Join("../../dev/node-example", name))
			if err != nil {
				t.Fatal(err)
			}
			if !valid && name == "package.json" {
				data = []byte(`{"scripts":{"build":"secret-command"}}`)
			}
			if err := writer.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR, Size: int64(len(data))}); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Write(data); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		input := filepath.Join(t.TempDir(), "example.tar")
		if err := os.WriteFile(input, archive.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		var out, diagnostics bytes.Buffer
		code := run(context.Background(), []string{"-archive", input, "-runtime", "node"}, &out, &diagnostics)
		if valid && (code != 0 || !strings.Contains(out.String(), `"files":4`)) {
			t.Fatalf("code=%d error=%s", code, &diagnostics)
		}
		if !valid && (code != 1 || diagnostics.String() != "node_contract_invalid\n" || out.Len() != 0) {
			t.Fatalf("unsafe failure: code=%d %s", code, &diagnostics)
		}
	}
}

func TestSourcecheckSavesLocalArtifact(t *testing.T) {
	if os.Getenv("SPROUT_DOCKER_TEST") != "1" {
		t.Skip("set SPROUT_DOCKER_TEST=1 to run the local Docker build")
	}
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	for _, name := range []string{"package.json", "package-lock.json", "build.mjs", "server.mjs"} {
		data, err := os.ReadFile(filepath.Join("../../dev/node-example", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := writer.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "example.tar")
	if err := os.WriteFile(input, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	code := run(context.Background(), []string{"-archive", input, "-build-node", "-artifact-dir", directory}, &out, &diagnostics)
	if code != 0 {
		t.Fatalf("local build failed: code=%d error=%s", code, &diagnostics)
	}
	var result struct {
		Files    int                 `json:"files"`
		Artifact artifact.Descriptor `json:"artifact"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Files != 4 {
		t.Fatalf("invalid build summary: %s (%v)", &out, err)
	}
	store, err := artifact.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	built, err := store.Load(context.Background(), result.Artifact)
	if err != nil || !bytes.Contains(built, []byte("A small Node.js app")) {
		t.Fatalf("stored build output unavailable: %v", err)
	}
}
