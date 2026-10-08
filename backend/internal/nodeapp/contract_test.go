package nodeapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func sample() (Manifest, lockfile) {
	manifest := Manifest{Name: "example", Version: "1.0.0", Private: true, Engines: map[string]string{"node": "24.x"}, Scripts: map[string]string{"build": "node build.mjs", "start": "node dist/server.mjs"}}
	lock := lockfile{Name: "example", Version: "1.0.0", LockfileVersion: 3, Packages: map[string]lockPackage{"": {Name: "example", Version: "1.0.0"}}}
	return manifest, lock
}
func filesFor(t *testing.T, m Manifest, l lockfile) fstest.MapFS {
	t.Helper()
	manifest, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	// Omit absent workspaces just as an ordinary package.json does.
	var raw map[string]any
	if err := json.Unmarshal(manifest, &raw); err != nil {
		t.Fatal(err)
	}
	if m.Workspaces == nil {
		delete(raw, "workspaces")
	}
	manifest, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return fstest.MapFS{"package.json": {Data: manifest}, "package-lock.json": {Data: lock}, "server.mjs": {Data: []byte("// not executed")}}
}
func TestNodeContract(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Manifest, *lockfile)
		valid  bool
	}{
		{"valid", func(*Manifest, *lockfile) {}, true},
		{"wrong node", func(m *Manifest, _ *lockfile) { m.Engines["node"] = "22.x" }, false},
		{"not private", func(m *Manifest, _ *lockfile) { m.Private = false }, false},
		{"missing start", func(m *Manifest, _ *lockfile) { delete(m.Scripts, "start") }, false},
		{"missing build", func(m *Manifest, _ *lockfile) { m.Scripts["build"] = " " }, false},
		{"workspaces", func(m *Manifest, _ *lockfile) { m.Workspaces = json.RawMessage(`["packages/*"]`) }, false},
		{"other manager", func(m *Manifest, _ *lockfile) { m.PackageManager = "pnpm@10.0.0" }, false},
		{"old lock", func(_ *Manifest, l *lockfile) { l.LockfileVersion = 2 }, false},
		{"root mismatch", func(_ *Manifest, l *lockfile) { l.Name = "another-app" }, false},
		{"missing root", func(_ *Manifest, l *lockfile) { delete(l.Packages, "") }, false},
		{"dependency mismatch", func(m *Manifest, _ *lockfile) { m.Dependencies = map[string]string{"example": "1.0.0"} }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, l := sample()
			tc.change(&m, &l)
			err := Validate(context.Background(), filesFor(t, m, l))
			if (err == nil) != tc.valid {
				t.Fatalf("got %v valid=%v", err, tc.valid)
			}
		})
	}
}
func TestDependencyPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, key, resolved, integrity string
		link, valid                    bool
	}{
		{"registry", "node_modules/example", "https://registry.npmjs.org/example/-/example-1.0.0.tgz", "", false, true},
		{"private network", "node_modules/example", "http://127.0.0.1/secret.tgz", "", false, false},
		{"other host", "node_modules/example", "https://example.com/archive.tgz", "", false, false},
		{"credentials", "node_modules/example", "https://token@registry.npmjs.org/example.tgz", "", false, false},
		{"query", "node_modules/example", "https://registry.npmjs.org/example.tgz?token=secret", "", false, false},
		{"port", "node_modules/example", "https://registry.npmjs.org:443/example.tgz", "", false, false},
		{"traversal", "node_modules/../example", "https://registry.npmjs.org/example.tgz", "", false, false},
		{"link", "node_modules/example", "https://registry.npmjs.org/example.tgz", "", true, false},
		{"integrity", "node_modules/example", "https://registry.npmjs.org/example.tgz", "sha512-invalid", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, l := sample()
			m.Dependencies = map[string]string{"example": "1.0.0"}
			root := l.Packages[""]
			root.Dependencies = m.Dependencies
			l.Packages[""] = root
			integrity := tc.integrity
			if integrity == "" {
				integrity = "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64))
			}
			l.Packages[tc.key] = lockPackage{Version: "1.0.0", Resolved: tc.resolved, Integrity: integrity, Link: tc.link}
			err := Validate(context.Background(), filesFor(t, m, l))
			if (err == nil) != tc.valid {
				t.Fatalf("got %v valid=%v", err, tc.valid)
			}
		})
	}
}
func TestNodeInputBoundaries(t *testing.T) {
	for _, name := range []string{"Dockerfile", "custom.Dockerfile", ".dockerignore", "node_modules/a/index.js", "dist/server.js", ".next/cache/data", "yarn.lock", "pnpm-lock.yaml"} {
		t.Run(name, func(t *testing.T) {
			m, l := sample()
			files := filesFor(t, m, l)
			files[name] = &fstest.MapFile{Data: []byte("private content")}
			if err := Validate(context.Background(), files); !errors.Is(err, ErrContract) {
				t.Fatal(err)
			}
		})
	}
	m, l := sample()
	for _, content := range []string{"{broken", "null", strings.Repeat(" ", 256<<10) + "{}"} {
		files := filesFor(t, m, l)
		files["package.json"] = &fstest.MapFile{Data: []byte(content)}
		if err := Validate(context.Background(), files); !errors.Is(err, ErrContract) {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Validate(ctx, filesFor(t, m, l)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestCommittedExample(t *testing.T) {
	if err := Validate(context.Background(), os.DirFS("../../dev/node-example")); err != nil {
		t.Fatal(err)
	}
}
