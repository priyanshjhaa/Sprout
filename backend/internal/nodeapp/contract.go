// Package nodeapp validates the initial Node application contract. Validation
// does not make package scripts or dependencies safe to execute.
package nodeapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/url"
	"path"
	"reflect"
	"strings"
)

var ErrContract = errors.New("node_contract_invalid")

type Manifest struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Private              bool              `json:"private"`
	Engines              map[string]string `json:"engines"`
	Scripts              map[string]string `json:"scripts"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	Workspaces           json.RawMessage   `json:"workspaces"`
	PackageManager       string            `json:"packageManager"`
}

type lockPackage struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Resolved             string            `json:"resolved"`
	Integrity            string            `json:"integrity"`
	Link                 bool              `json:"link"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}
type lockfile struct {
	Name            string                 `json:"name"`
	Version         string                 `json:"version"`
	LockfileVersion int                    `json:"lockfileVersion"`
	Packages        map[string]lockPackage `json:"packages"`
}

// Validate receives the filesystem from source.WithArchive. It never opens a
// network connection, installs packages, runs scripts, or returns source text.
func Validate(ctx context.Context, files fs.FS) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if files == nil {
		return ErrContract
	}
	var manifest Manifest
	var lock lockfile
	if err := readJSON(files, "package.json", 256<<10, &manifest); err != nil {
		return ErrContract
	}
	if err := readJSON(files, "package-lock.json", 1<<20, &lock); err != nil {
		return ErrContract
	}
	if manifest.Name == "" || manifest.Version == "" || !manifest.Private || manifest.Engines["node"] != "24.x" || len(manifest.Workspaces) != 0 ||
		(manifest.PackageManager != "" && !strings.HasPrefix(manifest.PackageManager, "npm@")) {
		return ErrContract
	}
	for _, name := range []string{"build", "start"} {
		if command := strings.TrimSpace(manifest.Scripts[name]); command == "" || len(command) > 2048 {
			return ErrContract
		}
	}
	if lock.LockfileVersion != 3 || lock.Name != manifest.Name || lock.Version != manifest.Version || len(lock.Packages) > 4096 {
		return ErrContract
	}
	root, ok := lock.Packages[""]
	if !ok || root.Name != manifest.Name || root.Version != manifest.Version || root.Link || root.Resolved != "" ||
		!sameDependencies(root.Dependencies, manifest.Dependencies) || !sameDependencies(root.DevDependencies, manifest.DevDependencies) || !sameDependencies(root.OptionalDependencies, manifest.OptionalDependencies) {
		return ErrContract
	}
	for _, dependencies := range []map[string]string{manifest.Dependencies, manifest.DevDependencies, manifest.OptionalDependencies} {
		for name, version := range dependencies {
			if !packageName(name) || version == "" || strings.ContainsAny(version, ":/@\\\r\n") {
				return ErrContract
			}
			if _, ok := lock.Packages["node_modules/"+name]; !ok {
				return ErrContract
			}
		}
	}
	for name, pkg := range lock.Packages {
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "" {
			continue
		}
		if !packagePath(name) || pkg.Link || pkg.Version == "" {
			return ErrContract
		}
		location, err := url.Parse(pkg.Resolved)
		if err != nil || location.Scheme != "https" || location.Host != "registry.npmjs.org" || location.User != nil || location.RawQuery != "" || location.Fragment != "" || location.Opaque != "" || !strings.HasSuffix(location.Path, ".tgz") {
			return ErrContract
		}
		digest, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(pkg.Integrity, "sha512-"))
		if err != nil || !strings.HasPrefix(pkg.Integrity, "sha512-") || len(digest) != 64 {
			return ErrContract
		}
	}
	count := 0
	err := fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return ErrContract
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		if count > 4096 {
			return ErrContract
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return ErrContract
		}
		for _, part := range strings.Split(strings.ToLower(name), "/") {
			if part == "node_modules" || part == ".next" || part == "dist" || part == ".npmrc" || part == ".yarnrc" || part == ".yarnrc.yml" || part == "yarn.lock" || part == "pnpm-lock.yaml" || part == ".dockerignore" || part == "dockerfile" || strings.HasSuffix(part, ".dockerfile") {
				return ErrContract
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return ctx.Err()
}

func sameDependencies(a, b map[string]string) bool {
	return len(a) == 0 && len(b) == 0 || reflect.DeepEqual(a, b)
}

func readJSON(files fs.FS, name string, limit int64, target any) error {
	file, err := files.Open(name)
	if err != nil {
		return ErrContract
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return ErrContract
	}
	decoder := json.NewDecoder(io.LimitReader(file, limit+1))
	if err := decoder.Decode(target); err != nil {
		return ErrContract
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return ErrContract
	}
	return nil
}

func packageName(name string) bool {
	parts := strings.Split(name, "/")
	if strings.HasPrefix(name, "@") {
		if len(parts) != 2 || len(parts[0]) < 2 {
			return false
		}
		parts[0] = strings.TrimPrefix(parts[0], "@")
	} else if len(parts) != 1 {
		return false
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
		for _, r := range part {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
				return false
			}
		}
	}
	return true
}
func packagePath(name string) bool {
	if path.Clean(name) != name {
		return false
	}
	parts := strings.Split(name, "/node_modules/")
	if !strings.HasPrefix(parts[0], "node_modules/") {
		return false
	}
	parts[0] = strings.TrimPrefix(parts[0], "node_modules/")
	for _, part := range parts {
		if !packageName(part) {
			return false
		}
	}
	return true
}
