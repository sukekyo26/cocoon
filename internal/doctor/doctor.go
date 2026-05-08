// Package doctor implements environment diagnostics for workspace-docker.
//
// It mirrors lib/doctor.sh but executes entirely in Go so the bash entry
// scripts can shell out to a single `wsd doctor` invocation. The output
// format (per-check ✓/⚠/✗ markers, ANSI colour codes, summary footer) is
// kept byte-equivalent to the legacy implementation so existing consumers
// (snapshot tests, CI logs, end-user expectations) are not surprised.
package doctor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sukekyo26/cocoon/internal/certificates"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/dockersock"
	wsdexec "github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/exec/dockerx"
	"github.com/sukekyo26/cocoon/internal/plugin"
	"github.com/sukekyo26/cocoon/internal/repositories"
)

const (
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

// Reporter accumulates pass/warn/fail counters and writes per-check output.
type Reporter struct {
	w      io.Writer
	runner wsdexec.Runner
	docker *dockerx.Client
	pass   int
	warn   int
	fail   int
}

// NewReporter returns a Reporter that writes to w using the real exec runner.
func NewReporter(w io.Writer) *Reporter {
	r := wsdexec.New()
	return &Reporter{w: w, runner: r, docker: dockerx.New(r), pass: 0, warn: 0, fail: 0}
}

// NewReporterWithRunner returns a Reporter that delegates external commands
// to runner. Tests use this to inject a [wsdexec.RecordingRunner].
func NewReporterWithRunner(w io.Writer, runner wsdexec.Runner) *Reporter {
	return &Reporter{w: w, runner: runner, docker: dockerx.New(runner), pass: 0, warn: 0, fail: 0}
}

// Pass records a successful check and emits a green ✓ line.
func (r *Reporter) Pass(msg string) {
	fmt.Fprintf(r.w, "  %s[✓]%s %s\n", colorGreen, colorReset, msg)
	r.pass++
}

// Warn records a warning and emits a yellow ⚠ line.
func (r *Reporter) Warn(msg string, hints ...string) {
	fmt.Fprintf(r.w, "  %s[⚠]%s %s\n", colorYellow, colorReset, msg)
	for _, h := range hints {
		fmt.Fprintf(r.w, "      → %s\n", h)
	}
	r.warn++
}

// Fail records a failure and emits a red ✗ line. Additional hint strings
// (typically remediation commands) are printed indented under the check.
func (r *Reporter) Fail(msg string, hints ...string) {
	fmt.Fprintf(r.w, "  %s[✗]%s %s\n", colorRed, colorReset, msg)
	for _, h := range hints {
		fmt.Fprintf(r.w, "      → %s\n", h)
	}
	r.fail++
}

// Summary writes the trailing pass/warn/fail counters.
func (r *Reporter) Summary() {
	fmt.Fprintln(r.w)
	fmt.Fprintf(r.w, "Summary: %d passed, %d warning, %d failed\n", r.pass, r.warn, r.fail)
}

// HasFailures reports whether at least one Fail was recorded.
func (r *Reporter) HasFailures() bool { return r.fail > 0 }

// Options selects which checks Run executes. The zero value runs everything.
type Options struct {
	Root       string         // workspace root (defaults to cwd)
	PluginsDir string         // override plugins dir; defaults to <Root>/plugins
	Runner     wsdexec.Runner // overrides the exec runner; defaults to exec.New()
}

// Run executes the full doctor check sequence and writes results to w.
// Returns true when no failures were recorded (callers should propagate
// that to exit code 0/1).
func Run(opts Options, w io.Writer) bool {
	root := opts.Root
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}
	pluginsDir := opts.PluginsDir
	if pluginsDir == "" {
		pluginsDir = filepath.Join(root, "plugins")
	}

	runner := opts.Runner
	if runner == nil {
		runner = wsdexec.New()
	}
	r := NewReporterWithRunner(w, runner)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "workspace-docker doctor")
	fmt.Fprintln(w, "========================")

	checkBashVersion(r)
	checkUV(r)
	checkDocker(r)
	checkComposePlugin(r)
	checkWorkspaceTOML(r, root)
	checkPluginTOMLs(r, pluginsDir)
	checkComposeSyntax(r, root)
	checkInitGuard(r, root)
	checkDockerfileBuildKit(r, root)
	checkDockerSock(r, root)
	checkSidecars(r, root)
	checkRepositories(r, root)
	checkCertificates(r, root)

	r.Summary()
	return !r.HasFailures()
}

func checkBashVersion(r *Reporter) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := r.runner.Output(ctx, "bash", "-c", `echo "${BASH_VERSINFO[0]}.${BASH_VERSINFO[1]}"`)
	if err != nil {
		r.Fail("Bash version probe failed", "Install bash >= 4.3")
		return
	}
	v := strings.TrimSpace(string(out))
	parts := strings.SplitN(v, ".", 2)
	if len(parts) != 2 {
		r.Fail("Bash version unrecognised: "+v, "Install bash >= 4.3")
		return
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		r.Fail("Bash version unrecognised: "+v, "Install bash >= 4.3")
		return
	}
	if major > 4 || (major == 4 && minor >= 3) {
		r.Pass(fmt.Sprintf("Bash version ≥ 4.3 (current: %s)", v))
	} else {
		r.Fail(fmt.Sprintf("Bash version ≥ 4.3 required (current: %s)", v),
			"Install a newer bash or run via bash >= 4.3")
	}
}

func checkUV(r *Reporter) {
	if _, err := exec.LookPath("uv"); err != nil {
		r.Fail("uv is not installed",
			"Install uv: https://docs.astral.sh/uv/getting-started/installation/")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := r.runner.Output(ctx, "uv", "--version")
	v := "unknown"
	if err == nil {
		if line := firstLine(string(out)); line != "" {
			v = line
		}
	}
	r.Pass(fmt.Sprintf("uv is installed (%s)", v))
}

func checkDocker(r *Reporter) {
	switch diagnoseDockerFailure(r) {
	case "ok":
		r.Pass("Docker daemon reachable")
	case "not_installed":
		r.Fail("docker CLI not found",
			"Install: curl -fsSL https://get.docker.com -o get-docker.sh && sudo sh get-docker.sh")
	case "permission_denied":
		r.Fail("Docker daemon not reachable (permission denied)",
			`Add user to docker group: sudo usermod -aG docker $USER && newgrp docker`)
	default:
		r.Fail("Docker daemon not reachable",
			"Start Docker: sudo systemctl start docker")
	}
}

func diagnoseDockerFailure(r *Reporter) string {
	if _, err := exec.LookPath("docker"); err != nil {
		return "not_installed"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := r.docker.InfoCombinedOutput(ctx)
	if err == nil {
		return "ok"
	}
	s := string(out)
	if strings.Contains(s, "permission denied") || strings.Contains(s, "Permission denied") {
		return "permission_denied"
	}
	return "not_running"
}

func checkComposePlugin(r *Reporter) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.docker.ComposeVersion(ctx); err != nil {
		r.Fail("docker compose plugin not available",
			"Install the Docker Compose V2 plugin")
		return
	}
	v := "?"
	if s, err := r.docker.ComposeVersionShort(ctx); err == nil && s != "" {
		v = s
	}
	r.Pass(fmt.Sprintf("docker compose plugin available (v%s)", v))
}

func checkWorkspaceTOML(r *Reporter, root string) {
	f := filepath.Join(root, "workspace.toml")
	if !fileExists(f) {
		r.Fail(fmt.Sprintf("workspace.toml not found at %s", f),
			"Run ./setup-docker.sh --init to bootstrap")
		return
	}
	r.Pass("workspace.toml exists")
}

func checkPluginTOMLs(r *Reporter, pluginsDir string) {
	if !dirExists(pluginsDir) {
		r.Warn("plugins/ directory missing (skipping plugin TOML validation)")
		return
	}
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		r.Fail("Cannot read plugins directory: " + err.Error())
		return
	}
	var failures []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		f := filepath.Join(pluginsDir, e.Name(), "plugin.toml")
		if !fileExists(f) {
			continue
		}
		if _, err := plugin.Load(f); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		r.Fail("Invalid plugin TOML detected", failures...)
		return
	}
	r.Pass("All plugin TOML files parse and validate against schema")
}

func checkComposeSyntax(r *Reporter, root string) {
	f := filepath.Join(root, "docker-compose.yml")
	if !fileExists(f) {
		r.Warn("docker-compose.yml not generated yet (run ./setup-docker.sh)")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	env := append(os.Environ(),
		"OS_IMAGE=ubuntu",
		"OS_VERSION=24.04",
		"USERNAME=dev",
		"UID_=1000",
		"GID=1000",
		"DOCKER_GID=999",
		"CONTAINER_SERVICE_NAME=dev",
		"COMPOSE_PROJECT_NAME=ws",
	)
	err := r.runner.RunWithIO(ctx, wsdexec.RunOptions{
		Name:   "docker",
		Args:   []string{"compose", "-f", f, "config", "-q"},
		Stdin:  nil,
		Stdout: nil,
		Stderr: nil,
		Env:    env,
		Dir:    root,
	})
	if err != nil {
		r.Warn("docker compose config reported issues (may be due to unset env vars)")
		return
	}
	r.Pass("docker-compose.yml syntax valid")
}

func checkInitGuard(r *Reporter, root string) {
	f := filepath.Join(root, "docker-compose.yml")
	if !fileExists(f) {
		return
	}
	data, err := os.ReadFile(f) //nolint:gosec // path under workspace root
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(t, "init:") && strings.Contains(t, "true") {
			r.Pass("docker-compose.yml has init: true (PID exhaustion guard)")
			return
		}
	}
	r.Fail("docker-compose.yml missing 'init: true'",
		"Regenerate via ./setup-docker.sh to apply PID-exhaustion guard")
}

func checkDockerfileBuildKit(r *Reporter, root string) {
	f := filepath.Join(root, "Dockerfile")
	if !fileExists(f) {
		return
	}
	data, err := os.ReadFile(f) //nolint:gosec // path under workspace root
	if err != nil {
		return
	}
	first := firstLine(string(data))
	if strings.HasPrefix(first, "# syntax=docker/dockerfile") {
		r.Pass("Dockerfile uses BuildKit syntax directive")
		return
	}
	r.Warn("Dockerfile missing BuildKit syntax directive (regenerate to enable cache mounts)")
}

func checkDockerSock(r *Reporter, root string) {
	f := filepath.Join(root, "workspace.toml")
	if !fileExists(f) {
		return
	}
	ws, err := config.LoadWorkspace(f)
	if err != nil {
		return
	}
	hasDockerCLI := false
	for _, p := range ws.Plugins.Enable {
		if p == "docker-cli" {
			hasDockerCLI = true
			break
		}
	}
	if !hasDockerCLI {
		return
	}
	sock := dockersock.First()
	if sock == "" {
		r.Warn("Docker socket not found (docker-cli plugin enabled)",
			"Looked at: "+strings.Join(dockersock.CandidatePaths(), ", "),
			`On macOS Docker Desktop, enable Settings → Advanced → "Allow the default Docker socket to be used"`)
		return
	}
	if isReadWritable(sock) {
		r.Pass(sock + " accessible")
	} else {
		r.Warn(sock+" exists but not r/w by current user",
			`Add your user to the docker group: sudo usermod -aG docker $USER`)
	}
}

func checkSidecars(r *Reporter, root string) {
	f := filepath.Join(root, "workspace.toml")
	if !fileExists(f) {
		return
	}
	ws, err := config.LoadWorkspace(f)
	if err != nil || len(ws.Services) == 0 {
		return
	}
	if _, err := exec.LookPath("docker"); err != nil {
		r.Warn("Sidecars defined but docker CLI not available; skipping health probe")
		return
	}

	names := make([]string, 0, len(ws.Services))
	for name := range ws.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	psLines := composePSLines(r, root)
	for _, sc := range names {
		reportSidecar(r, sc, psLines[sc])
	}
}

func composePSLines(r *Reporter, root string) map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var buf bytes.Buffer
	err := r.runner.RunWithIO(ctx, wsdexec.RunOptions{
		Name:   "docker",
		Args:   []string{"compose", "ps", "--format", "{{.Service}}\t{{.State}}\t{{.Health}}"},
		Stdin:  nil,
		Stdout: &buf,
		Stderr: nil,
		Env:    nil,
		Dir:    root,
	})
	if err != nil {
		return map[string]string{}
	}
	res := map[string]string{}
	for _, line := range strings.Split(buf.String(), "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 2 {
			continue
		}
		res[fields[0]] = line
	}
	return res
}

func reportSidecar(r *Reporter, name, line string) {
	if line == "" {
		r.Warn(fmt.Sprintf("[services.%s] container not running", name),
			"Start it with: docker compose up -d "+name)
		return
	}
	fields := strings.SplitN(line, "\t", 3)
	state := fields[1]
	health := ""
	if len(fields) >= 3 {
		health = fields[2]
	}
	label := fmt.Sprintf("[services.%s] state=%s", name, state)
	if health != "" && health != "<no value>" {
		label += " health=" + health
	}
	switch health {
	case "healthy":
		r.Pass(label)
	case "starting":
		r.Warn(label)
	case "unhealthy":
		r.Fail(label, "Inspect logs: docker compose logs "+name)
	default:
		if state == "running" {
			r.Pass(label)
		} else {
			r.Warn(label)
		}
	}
}

func checkRepositories(r *Reporter, root string) {
	reports, err := repositories.CheckStatus(root)
	if err != nil || len(reports) == 0 {
		return
	}
	for _, rep := range reports {
		label := "[repositories] " + rep.Path
		switch rep.Status {
		case repositories.StatusOK:
			r.Pass(label)
		case repositories.StatusMissing:
			r.Warn(label+" missing", "Run ./setup-docker.sh to clone "+rep.URL)
		case repositories.StatusNotGit:
			r.Warn(label + " exists but is not a git repo")
		case repositories.StatusBadPath:
			r.Fail(label + " has unsafe path (escapes parent)")
		}
	}
}

func checkCertificates(r *Reporter, root string) {
	dir := filepath.Join(root, "certs")
	if !dirExists(dir) {
		return
	}
	list, err := certificates.List(root)
	if err != nil {
		r.Warn("Unable to scan certs/: " + err.Error())
		return
	}
	if len(list) == 0 {
		r.Warn("certs/ directory exists but contains no valid PEM certificates")
		return
	}
	r.Pass(fmt.Sprintf("certs/ has %d valid certificate(s)", len(list)))
}

// ---------------- helpers ----------------

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func isReadWritable(p string) bool {
	rf, err := os.OpenFile(p, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	_ = rf.Close()
	wf, err := os.OpenFile(p, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = wf.Close()
	return true
}
