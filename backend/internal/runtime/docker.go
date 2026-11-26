package runtime

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	networkName    = "sprout-runtime"
	appPort        = "3000"
	labelPurpose   = "io.sprout.purpose"
	labelApp       = "io.sprout.application"
	labelRelease   = "io.sprout.deployment"
	purposeApp     = "app-runtime"
	purposeRelease = "app-release"
	nonRoot        = "65532:65532"
)

var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Docker runs releases as containers on the local Docker daemon through the
// Docker CLI, the same trusted boundary the build runner uses. It is local
// developer infrastructure: the daemon must be one the operator trusts.
// Each runtime runs in the pinned image its release was built with.
type Docker struct{ node, python string }

func NewDocker(node, python string) (*Docker, error) {
	if !strings.HasPrefix(node, "node@sha256:") || !strings.HasPrefix(python, "python@sha256:") {
		return nil, ErrInvalid
	}
	return &Docker{node: node, python: python}, nil
}

// image is the release's runtime image. An empty runtime is Node, the
// database's default for deployments made before runtimes were recorded.
func (d *Docker) image(r Release) (string, bool) {
	switch r.Runtime {
	case RuntimeNode, "":
		return d.node, true
	case RuntimePython:
		return d.python, true
	}
	return "", false
}

func valid(r Release) bool {
	return uuid.MatchString(r.ApplicationID) && uuid.MatchString(r.DeploymentID)
}

func volumeName(r Release) string    { return "sprout-release-" + r.DeploymentID }
func containerName(r Release) string { return "sprout-app-" + r.DeploymentID }

func (d *Docker) Prepare(ctx context.Context, release Release, archive []byte) error {
	if _, known := d.image(release); !valid(release) || !known || len(archive) == 0 {
		return ErrInvalid
	}
	if err := docker(ctx, nil, nil, volumeCreateArgs(release)...); err != nil {
		return ErrUnavailable
	}
	if err := docker(ctx, bytes.NewReader(archive), nil, d.prepareArgs(release)...); err != nil {
		_ = d.Destroy(context.WithoutCancel(ctx), release)
		return ErrStart
	}
	return nil
}

func (d *Docker) Start(ctx context.Context, release Release) (Instance, error) {
	if _, known := d.image(release); !valid(release) || !known {
		return Instance{}, ErrInvalid
	}
	if err := ensureNetwork(ctx); err != nil {
		return Instance{}, ErrUnavailable
	}
	if err := docker(ctx, nil, nil, d.runArgs(release)...); err != nil {
		_ = d.Stop(context.WithoutCancel(ctx), release)
		return Instance{}, ErrStart
	}
	endpoint, err := d.Endpoint(ctx, release)
	if err != nil {
		_ = d.Stop(context.WithoutCancel(ctx), release)
		return Instance{}, ErrStart
	}
	return Instance{Release: release, Endpoint: endpoint}, nil
}

func (d *Docker) Endpoint(ctx context.Context, release Release) (string, error) {
	if !valid(release) {
		return "", ErrInvalid
	}
	var out bytes.Buffer
	if err := docker(ctx, nil, &out, "port", containerName(release), appPort+"/tcp"); err != nil {
		return "", ErrNotRunning
	}
	if endpoint := loopbackEndpoint(out.String()); endpoint != "" {
		return endpoint, nil
	}
	return "", ErrNotRunning
}

func (d *Docker) Stop(ctx context.Context, release Release) error {
	if !valid(release) {
		return ErrInvalid
	}
	return removeIfPresent(ctx, "container", containerName(release))
}

func (d *Docker) Destroy(ctx context.Context, release Release) error {
	if !valid(release) {
		return ErrInvalid
	}
	return errors.Join(removeIfPresent(ctx, "container", containerName(release)), removeIfPresent(ctx, "volume", volumeName(release)))
}

func (d *Docker) List(ctx context.Context) (Inventory, error) {
	var inventory Inventory
	var out bytes.Buffer
	format := `{{.Label "` + labelApp + `"}} {{.Label "` + labelRelease + `"}}`
	if err := docker(ctx, nil, &out, "ps", "-a", "--filter", "label="+labelPurpose+"="+purposeApp, "--format", format); err != nil {
		return Inventory{}, ErrUnavailable
	}
	inventory.Containers = parseReleases(out.String())
	out.Reset()
	if err := docker(ctx, nil, &out, "volume", "ls", "--filter", "label="+labelPurpose+"="+purposeRelease, "--format", format); err != nil {
		return Inventory{}, ErrUnavailable
	}
	inventory.Releases = parseReleases(out.String())
	return inventory, nil
}

func volumeCreateArgs(r Release) []string {
	return []string{"volume", "create", "--label", labelPurpose + "=" + purposeRelease,
		"--label", labelApp + "=" + r.ApplicationID, "--label", labelRelease + "=" + r.DeploymentID, volumeName(r)}
}

// prepareArgs unpacks the archive into the release volume. The helper has no
// network, no capabilities and a read-only root; it is root only so it may
// write the new volume, then makes the files world-readable and read-only for
// the unprivileged runtime user. A Node release with packages arrives without
// node_modules/.bin (symlinks never leave a build), so npm relinks it here,
// offline and without running any package's scripts. A Python release needs
// nothing beyond unpacking: its packages are plain files in site-packages/.
func (d *Docker) prepareArgs(r Release) []string {
	image, _ := d.image(r)
	command := prepareCommand
	if r.Runtime == RuntimePython {
		command = preparePython
	}
	return []string{"run", "--rm", "--interactive", "--pull=never", "--log-driver=none",
		"--name", "sprout-prepare-" + r.DeploymentID, "--label", labelPurpose + "=release-prepare",
		"--network", "none", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--cpus", "1", "--memory", "256m", "--memory-swap", "256m", "--pids-limit", "32",
		"--tmpfs", "/tmp:rw,nosuid,nodev,size=67108864,mode=0700",
		"--env", "HOME=/tmp", "--env", "npm_config_cache=/tmp/npm-cache", "--env", "npm_config_update_notifier=false",
		"--mount", "type=volume,src=" + volumeName(r) + ",dst=/app",
		image, "sh", "-c", command}
}

const prepareCommand = `set -e
tar -x --no-same-owner --no-same-permissions -C /app
if [ -d /app/node_modules ]; then
  cd /app && npm rebuild --offline --ignore-scripts --no-audit --no-fund >/dev/null 2>&1
fi
chmod -R a+rX,a-w /app`

// preparePython also marks the packages' commands (gunicorn, uvicorn)
// executable: Sprout repacks every build output with fixed file modes, so
// they arrive as plain files. That grants nothing new, since the application
// can already run any Python code.
const preparePython = `set -e
tar -x --no-same-owner --no-same-permissions -C /app
if [ -d /app/site-packages/bin ]; then
  find /app/site-packages/bin -maxdepth 1 -type f -exec chmod a+x {} +
fi
chmod -R a+rX,a-w /app`

// startPython runs the Procfile's web command, which the build already
// checked is the file's only process. Like npm start, it is the
// application's own shell command, run inside the application's sandbox.
const startPython = `exec sh -c "$(sed -n 's/^[[:space:]]*web[[:space:]]*:[[:space:]]*//p' /app/Procfile | tr -d '\r' | head -n 1)"`

// runArgs is the whole security posture of a running application. Application
// code is untrusted: no capabilities, no new privileges, a read-only root,
// the release mounted read-only, bounded CPU/memory/processes, a port
// published on loopback only, and a network on which apps cannot reach each
// other. Nothing here is chosen by the application.
func (d *Docker) runArgs(r Release) []string {
	image, _ := d.image(r)
	args := []string{"run", "--detach", "--init", "--pull=never", "--restart", "no", "--log-driver=none",
		"--name", containerName(r),
		"--label", labelPurpose + "=" + purposeApp, "--label", labelApp + "=" + r.ApplicationID, "--label", labelRelease + "=" + r.DeploymentID,
		"--user", nonRoot, "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--cpus", "0.5", "--memory", "256m", "--memory-swap", "256m", "--pids-limit", "64",
		"--tmpfs", "/tmp:rw,nosuid,nodev,size=67108864,uid=65532,gid=65532,mode=0700",
		"--mount", "type=volume,src=" + volumeName(r) + ",dst=/app,readonly",
		"--workdir", "/app", "--network", networkName, "--publish", "127.0.0.1::" + appPort,
		"--env", "PORT=" + appPort, "--env", "HOME=/tmp",
	}
	if r.Runtime == RuntimePython {
		// Packages live in the release's site-packages/, and their commands
		// (gunicorn, uvicorn) in its bin/. The release is read-only, so Python
		// is told not to try writing bytecode.
		return append(args, "--env", "PYTHONPATH=/app/site-packages", "--env", "PYTHONDONTWRITEBYTECODE=1", "--env", "PYTHONUNBUFFERED=1",
			"--env", "PATH=/app/site-packages/bin:/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin",
			image, "sh", "-c", startPython)
	}
	return append(args, "--env", "NODE_ENV=production",
		"--env", "npm_config_cache=/tmp/npm-cache", "--env", "npm_config_update_notifier=false",
		image, "npm", "--ignore-scripts", "start")
}

// ensureNetwork creates the shared app network once. Inter-container traffic
// is disabled so one untrusted app cannot probe another; outbound traffic
// stays allowed because real apps call other services.
func ensureNetwork(ctx context.Context) error {
	if docker(ctx, nil, nil, "network", "inspect", networkName) == nil {
		return nil
	}
	err := docker(ctx, nil, nil, "network", "create", "--driver", "bridge",
		"--opt", "com.docker.network.bridge.enable_icc=false", "--label", labelPurpose+"=app-network", networkName)
	if err != nil && docker(ctx, nil, nil, "network", "inspect", networkName) == nil {
		return nil // lost a creation race
	}
	return err
}

func removeIfPresent(ctx context.Context, kind, name string) error {
	args := []string{kind, "rm"}
	if kind == "container" {
		args = append(args, "--force")
	}
	args = append(args, "--", name)
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stderr = &stderr
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Run(); err != nil && !strings.Contains(strings.ToLower(stderr.String()), "no such") {
		return ErrUnavailable
	}
	return nil
}

func docker(ctx context.Context, stdin io.Reader, stdout io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, io.Discard
	cmd.WaitDelay = 5 * time.Second
	return cmd.Run()
}

// loopbackEndpoint picks the 127.0.0.1 mapping from `docker port` output.
func loopbackEndpoint(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "127.0.0.1:") {
			return line
		}
	}
	return ""
}

func parseReleases(output string) []Release {
	var releases []Release
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && valid(Release{ApplicationID: fields[0], DeploymentID: fields[1]}) {
			releases = append(releases, Release{ApplicationID: fields[0], DeploymentID: fields[1]})
		}
	}
	return releases
}
