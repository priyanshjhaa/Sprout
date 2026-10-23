// Package sourcepack turns a folder on this machine into the source archive the
// deploy API accepts: a plain USTAR tar of regular files. It is used by the CLI
// and, later, the MCP deploy tool; the dashboard does the same in the browser
// (frontend/src/lib/folder-archive.ts).
//
// The server stays the authority and re-checks everything. Packing only leaves
// out what nobody means to publish (hidden files, secrets, dependencies) and
// explains problems in plain words before an upload. Check then runs the
// server's own source stage on the result, so the rules cannot drift apart.
package sourcepack

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/source"
	"github.com/priyanshjhaa/Sprout/backend/internal/staticsite"
)

// Problem is something the person can fix. Its message is written for them.
type Problem struct{ Message string }

func (p *Problem) Error() string { return p.Message }

func problemf(format string, args ...any) error {
	return &Problem{Message: fmt.Sprintf(format, args...)}
}

// Kind is what the server will treat the folder as: a package.json at the top
// makes a Node.js app; otherwise it must be a static site with an index.html.
type Kind string

const (
	KindStatic Kind = "static"
	KindNode   Kind = "node"
)

// Archive is a packed folder, ready to upload.
type Archive struct {
	Tar   []byte
	Kind  Kind
	Files int
	Bytes int64
	// Skipped names each left-out file or folder, relative to the folder.
	Skipped []Skip
	// BuiltFolder is "dist", "build" or "out" when that folder holds a finished
	// website, which is often what someone meant to deploy.
	BuiltFolder string
}

type Skip struct {
	Path   string
	Reason Reason
}

type Reason string

const (
	ReasonHidden       Reason = "hidden"       // .env, .git, .DS_Store and every other dot-file
	ReasonPrivate      Reason = "private"      // keys and credential files
	ReasonDependencies Reason = "dependencies" // node_modules
	ReasonJunk         Reason = "junk"         // system clutter
	ReasonLink         Reason = "link"         // symlinks, which could point anywhere
	ReasonSpecial      Reason = "special"      // sockets, devices and pipes
)

var (
	secretNames    = []string{"credentials.json", "id_rsa", "id_ed25519", "id_ecdsa"}
	junkNames      = []string{"__macosx", "thumbs.db", "desktop.ini"}
	secretSuffixes = []string{".pem", ".key", ".p12", ".pfx"}
)

// SkipReason says why a single path segment is never uploaded, or "" if it is
// kept. It matches skipReason in the browser packer.
func SkipReason(segment string) Reason {
	lower := strings.ToLower(segment)
	switch {
	case strings.HasPrefix(lower, "._"): // macOS resource forks
		return ReasonJunk
	case strings.HasPrefix(lower, "."):
		return ReasonHidden
	case lower == "node_modules":
		return ReasonDependencies
	case slices.Contains(junkNames, lower):
		return ReasonJunk
	case slices.Contains(secretNames, lower) || slices.ContainsFunc(secretSuffixes, func(suffix string) bool { return strings.HasSuffix(lower, suffix) }):
		return ReasonPrivate
	}
	return ""
}

type entry struct {
	path string
	size int64
}

// Pack reads dir and packs what should be deployed. Symlinks are never
// followed, and every file is opened through os.Root, so nothing outside dir
// can be read even if the folder changes while it is being packed.
func Pack(ctx context.Context, dir string) (Archive, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return Archive{}, problemf("cannot open %s: %v", dir, err)
	}
	defer root.Close()

	var result Archive
	var files []entry
	err = fs.WalkDir(root.FS(), ".", func(name string, item fs.DirEntry, err error) error {
		if err != nil {
			return problemf("cannot read %q: %v", name, err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		reason := SkipReason(item.Name())
		switch {
		case reason != "":
		case item.Type()&fs.ModeSymlink != 0:
			reason = ReasonLink
		case !item.IsDir() && !item.Type().IsRegular():
			reason = ReasonSpecial
		}
		if reason != "" {
			result.Skipped = append(result.Skipped, Skip{Path: name, Reason: reason})
			if item.IsDir() {
				return fs.SkipDir // never even opened, so node_modules costs nothing
			}
			return nil
		}
		if item.IsDir() {
			return nil
		}
		if len(files) == source.MaxEntries {
			return problemf("this folder has more than %d files; Sprout takes up to %d. Remove what the app doesn't need.", source.MaxEntries, source.MaxEntries)
		}
		info, err := item.Info()
		if err != nil {
			return problemf("cannot read %q: %v", name, err)
		}
		files = append(files, entry{path: name, size: info.Size()})
		return nil
	})
	if err != nil {
		return Archive{}, err
	}
	if err := checkFiles(files, result.Skipped); err != nil {
		return Archive{}, err
	}
	if result.Kind, err = kindOf(files); err != nil {
		return Archive{}, err
	}
	if result.Kind == KindStatic {
		if err := checkWebNames(files); err != nil {
			return Archive{}, err
		}
	}
	result.BuiltFolder = builtFolder(files)

	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return Archive{}, err
		}
		if err := writeFile(writer, root, file); err != nil {
			return Archive{}, err
		}
		result.Files++
		result.Bytes += file.size
	}
	if err := writer.Close(); err != nil {
		return Archive{}, err
	}
	result.Tar = buffer.Bytes()
	return result, nil
}

// checkFiles applies the server's limits and name rules, naming the files that
// break them.
func checkFiles(files []entry, skipped []Skip) error {
	if len(files) == 0 {
		if len(skipped) > 0 {
			return problemf("everything in this folder is hidden, private or a dependency (%s), so there is nothing to deploy.", listPaths(skipped))
		}
		return problemf("this folder is empty.")
	}
	var total int64
	var tooBig, badNames []string
	seen := map[string]bool{}
	for _, file := range files {
		total += file.size
		if file.size > source.MaxFileBytes {
			tooBig = append(tooBig, file.path)
		}
		if err := source.CheckName(file.path); err != nil {
			badNames = append(badNames, file.path)
		}
		key := strings.ToLower(file.path)
		if seen[key] {
			return problemf("%q appears twice with different capitalisation. Rename one.", file.path)
		}
		seen[key] = true
	}
	if len(tooBig) > 0 {
		return problemf("files over %s can't be deployed: %s.", formatBytes(source.MaxFileBytes), quoteList(tooBig))
	}
	if total > source.MaxSourceBytes {
		return problemf("this folder is %s; Sprout takes up to %s.", formatBytes(total), formatBytes(source.MaxSourceBytes))
	}
	if len(badNames) > 0 {
		return problemf("these paths can't be used: %s. Paths must be at most 16 folders deep and 240 bytes long, and names can't contain \\ or :, start or end with a space, or end with a dot.", quoteList(badNames))
	}
	return nil
}

func kindOf(files []entry) (Kind, error) {
	has := func(name string) bool {
		return slices.ContainsFunc(files, func(file entry) bool { return file.path == name })
	}
	if has("package.json") {
		return KindNode, nil
	}
	if has("index.html") {
		return KindStatic, nil
	}
	hint := ""
	for _, file := range files {
		if !strings.Contains(file.path, "/") && (strings.EqualFold(file.path, "index.htm") || strings.EqualFold(file.path, "index.html")) {
			hint = fmt.Sprintf("Rename %q to exactly \"index.html\". ", file.path)
			break
		}
	}
	if built := builtFolder(files); built != "" {
		hint += fmt.Sprintf("The finished website looks like it is in %q: deploy that folder instead. ", built)
	}
	var found []string
	for _, file := range files {
		if top, _, _ := strings.Cut(file.path, "/"); !slices.Contains(found, top) {
			found = append(found, top)
		}
	}
	return "", problemf("%sSprout needs an index.html (a website) or a package.json (a Node.js app) at the top of the folder. Found: %s.", hint, quoteList(found))
}

func builtFolder(files []entry) string {
	for _, built := range []string{"dist", "build", "out"} {
		if slices.ContainsFunc(files, func(file entry) bool { return file.path == built+"/index.html" }) {
			return built
		}
	}
	return ""
}

// checkWebNames applies the static-site name rule, which is stricter than the
// archive's: a website's paths become URLs.
func checkWebNames(files []entry) error {
	var bad []string
	for _, file := range files {
		if !staticsite.ValidName(file.path) {
			bad = append(bad, file.path)
		}
	}
	if len(bad) > 0 {
		return problemf("these file names can't be used on a website: %s. Use only letters, numbers, dots, dashes and underscores (no spaces).", quoteList(bad))
	}
	return nil
}

// writeFile copies one file into the archive with a fixed header: mode 0644,
// owner 0 and the Unix epoch as its time, so the same folder always packs to
// the same bytes. Directories are implied by paths, as the server expects.
func writeFile(writer *tar.Writer, root *os.Root, file entry) error {
	opened, err := root.Open(file.path)
	if err != nil {
		return problemf("%q could not be read: %v", file.path, err)
	}
	defer opened.Close()
	// Recheck the opened object: it must still be the regular file we listed.
	info, err := opened.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != file.size {
		return problemf("%q changed while it was being packed. Try again.", file.path)
	}
	header := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     file.path,
		Mode:     0o644,
		Size:     file.size,
		ModTime:  time.Unix(0, 0),
		Format:   tar.FormatUSTAR,
	}
	if err := writer.WriteHeader(header); err != nil {
		return problemf("%q can't be stored in a source archive: %v", file.path, err)
	}
	written, err := io.Copy(writer, io.LimitReader(opened, file.size))
	if err != nil {
		return problemf("%q could not be read: %v", file.path, err)
	}
	if written != file.size {
		return problemf("%q changed while it was being packed. Try again.", file.path)
	}
	return nil
}

func listPaths(skipped []Skip) string {
	paths := make([]string, len(skipped))
	for i, skip := range skipped {
		paths[i] = skip.Path
	}
	return quoteList(paths)
}

func quoteList(names []string) string {
	const max = 4
	quoted := make([]string, 0, max)
	for _, name := range names[:min(len(names), max)] {
		quoted = append(quoted, fmt.Sprintf("%q", name))
	}
	list := strings.Join(quoted, ", ")
	if len(names) > max {
		list += fmt.Sprintf(" and %d more", len(names)-max)
	}
	return list
}

func formatBytes(bytes int64) string {
	if bytes >= 1<<20 {
		return fmt.Sprintf("%.1f MiB", float64(bytes)/(1<<20))
	}
	return fmt.Sprintf("%d KiB", max(1, (bytes+512)>>10))
}

// IsProblem reports whether err is a Problem the person can fix.
func IsProblem(err error) bool {
	var problem *Problem
	return errors.As(err, &problem)
}
