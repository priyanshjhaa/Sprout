// Package pythonapp validates and builds Python web applications. Validation
// does not make the application's code or packages safe to run; it only
// checks that the project is one Sprout knows how to build in the sealed
// container and start in the runtime.
package pythonapp

import (
	"bufio"
	"context"
	"errors"
	"io"
	"io/fs"
	"strings"

	"github.com/priyanshjhaa/Sprout/backend/internal/pypifetch"
)

var ErrContract = errors.New("python_contract_invalid")

// ContractError is an ErrContract that says which rule failed. Code is a
// stable, user-safe identifier (never source text).
type ContractError struct{ Code string }

func (e *ContractError) Error() string        { return e.Code }
func (e *ContractError) Is(target error) bool { return target == ErrContract }

func broke(code string) error { return &ContractError{Code: code} }

// The rule that failed, as reported to the user.
const (
	reasonRequirements = "python_contract_requirements" // requirements.txt missing, or not pinned with hashes
	reasonProcfile     = "python_contract_procfile"     // Procfile missing, or not exactly one web: command
	reasonFiles        = "python_contract_files"        // files Sprout does not accept in the source
)

// MaxCommand bounds the web command.
const MaxCommand = 2048

// IsPython reports whether a source is a Python project: it has a
// requirements.txt at the top. A package.json takes precedence (checked by
// the caller first), because a Node project may carry Python helper files.
func IsPython(files fs.FS) bool {
	info, err := fs.Stat(files, "requirements.txt")
	return err == nil && info.Mode().IsRegular()
}

// Validate receives the filesystem from source.WithArchive. It never opens a
// network connection, installs packages or runs code.
func Validate(ctx context.Context, files fs.FS) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if files == nil {
		return ErrContract
	}
	requirements, err := readSmall(files, "requirements.txt", pypifetch.MaxRequirementsBytes)
	if err != nil {
		return broke(reasonRequirements)
	}
	if _, err := pypifetch.ParseRequirements(requirements); err != nil {
		return broke(reasonRequirements)
	}
	procfile, err := readSmall(files, "Procfile", 16<<10)
	if err != nil {
		return broke(reasonProcfile)
	}
	if _, err := WebCommand(procfile); err != nil {
		return err
	}
	count := 0
	err = fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return broke(reasonFiles)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if count++; count > 4096 || entry.Type()&fs.ModeSymlink != 0 {
			return broke(reasonFiles)
		}
		for _, part := range strings.Split(strings.ToLower(name), "/") {
			// site-packages is where the build installs packages, and a
			// pyvenv.cfg means a whole virtual environment was uploaded.
			if part == "site-packages" || part == "pyvenv.cfg" || part == "node_modules" || part == ".dockerignore" || part == "dockerfile" || strings.HasSuffix(part, ".dockerfile") {
				return broke(reasonFiles)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return ctx.Err()
}

// WebCommand reads a Procfile, the format Heroku introduced and buildpacks,
// Render and Railway also read: "web: <command>". Sprout runs only a web
// process, so any other process type is refused rather than silently ignored.
func WebCommand(procfile []byte) (string, error) {
	var command string
	found := false
	scanner := bufio.NewScanner(strings.NewReader(string(procfile)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kind, value, ok := strings.Cut(line, ":")
		value = strings.TrimSpace(value)
		if !ok || strings.TrimSpace(kind) != "web" || found || value == "" || len(value) > MaxCommand || strings.ContainsRune(value, 0) {
			return "", broke(reasonProcfile)
		}
		command, found = value, true
	}
	if scanner.Err() != nil || !found {
		return "", broke(reasonProcfile)
	}
	return command, nil
}

func readSmall(files fs.FS, name string, limit int64) ([]byte, error) {
	file, err := files.Open(name)
	if err != nil {
		return nil, ErrContract
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, ErrContract
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, ErrContract
	}
	return data, nil
}
