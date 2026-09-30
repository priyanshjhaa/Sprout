package main

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
