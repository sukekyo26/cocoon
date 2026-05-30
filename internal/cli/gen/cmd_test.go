package gencli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	gencli "github.com/sukekyo26/cocoon/internal/cli/gen"
)

// pinEnglish forces the i18n catalog to English for assertion stability.
// The cocoon binary picks Japanese when LANG / LC_* / WORKSPACE_LANG starts
// with "ja"; CI machines and Japanese developer hosts both fit that profile.
func pinEnglish(t *testing.T) {
	t.Helper()
	for _, k := range []string{"WORKSPACE_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(k, "")
	}
	t.Setenv("WORKSPACE_LANG", "en")
}

const minimalWorkspaceTOML = `[workspace]
mount_root = "."
devcontainer = true

[container]
service_name = "demo"
username = "alice"
image = "ubuntu"
image_version = "24.04"

[plugins]
enable = []

[apt]
packages = []
`

// TestGen_DefaultRun runs `cocoon gen` with no flags inside a tempdir
// containing workspace.toml. It asserts that the three .devcontainer/
// artifacts land on disk and that the next-step message points the user
// at docker compose.
func TestGen_DefaultRun(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(minimalWorkspaceTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen: %v\nstderr:\n%s", err, stderr.String())
	}

	for _, rel := range []string{
		".devcontainer/Dockerfile",
		".devcontainer/docker-compose.yml",
		".devcontainer/devcontainer.json",
		".devcontainer/manage.sh",
	} {
		if _, statErr := os.Stat(filepath.Join(work, rel)); statErr != nil {
			t.Errorf("expected %s to exist: %v", rel, statErr)
		}
	}

	if info, statErr := os.Stat(filepath.Join(work, ".devcontainer/manage.sh")); statErr == nil {
		if perm := info.Mode().Perm(); perm != 0o755 {
			t.Errorf("manage.sh mode = %o, want 0755", perm)
		}
	}

	out := stdout.String()
	for _, want := range []string{
		"wrote .devcontainer/Dockerfile",
		"wrote .devcontainer/docker-compose.yml",
		"wrote .devcontainer/devcontainer.json",
		"wrote .devcontainer/manage.sh",
		"To start the container:",
		"docker compose -f .devcontainer/docker-compose.yml up -d",
		`Reopen in Container`,
		"./.devcontainer/manage.sh -h",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\n--- got ---\n%s", want, out)
		}
	}
}

// TestGen_LoadFailureHasSingleFailurePrefix pins that a workspace load
// failure surfaces with a single "failure:" prefix. LoadContext already
// wraps with ErrFailure, so the caller must not wrap again (defensive-coding
// §3: wrap once per call chain).
func TestGen_LoadFailureHasSingleFailurePrefix(t *testing.T) {
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	// Invalid service_name → workspace validation fails inside LoadContext.
	bad := strings.ReplaceAll(minimalWorkspaceTOML, `service_name = "demo"`, `service_name = "Demo"`)
	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(bad), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for an invalid workspace.toml")
	}
	if !errors.Is(err, clihelpers.ErrFailure) {
		t.Errorf("expected ErrFailure, got %v", err)
	}
	if strings.Contains(err.Error(), "failure: failure") {
		t.Errorf("doubled \"failure:\" prefix in error: %q", err.Error())
	}
}

// certsEnabledWorkspaceTOML is `minimalWorkspaceTOML` plus an explicit
// `[certificates] enable = true` so cert auto-bake (mkdir + notice) is
// turned on for the test.
const certsEnabledWorkspaceTOML = minimalWorkspaceTOML + `
[certificates]
enable = true
`

// TestGen_CreatesUserCertsDir verifies that `cocoon gen` creates
// ~/.cocoon/certs (mode 0700) on first run, surfaces the notice
// messages, and stays quiet about creation when the directory already
// exists — but only when `[certificates] enable = true` opts in. Without
// the section, gen never touches $HOME (covered by
// TestGen_CertificatesDisabled_SkipsMkdirAndNotice).
func TestGen_CreatesUserCertsDir(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(certsEnabledWorkspaceTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	certsDir := filepath.Join(home, ".cocoon", "certs")
	if _, statErr := os.Stat(certsDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("certs dir should not exist before gen: %v", statErr)
	}

	// First run: directory is created and the "created" line surfaces.
	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen #1: %v\nstderr:\n%s", err, stderr.String())
	}
	info, err := os.Stat(certsDir)
	if err != nil {
		t.Fatalf("certs dir should exist after gen: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s should be a directory", certsDir)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Errorf("certs dir mode = %o, want 0700", mode)
	}
	out := stdout.String()
	for _, want := range []string{
		"created host directory",
		"cocoon_user_certs build context",
		"Host TLS certificates:",
		"Drop *.crt files into ~/.cocoon/certs/",
		"Team members who skip VS Code Dev Containers",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("first-run stdout missing %q\n--- got ---\n%s", want, out)
		}
	}

	// Second run: directory already exists, "created" line must not
	// reappear, but the steady-state notice block still does.
	stdout.Reset()
	stderr.Reset()
	cmd2 := gencli.NewCommand(&stdout, &stderr)
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("gen #2: %v\nstderr:\n%s", err, stderr.String())
	}
	out = stdout.String()
	if strings.Contains(out, "created host directory") {
		t.Errorf("re-run should not announce directory creation:\n%s", out)
	}
	if !strings.Contains(out, "Host TLS certificates:") {
		t.Errorf("re-run should still emit the cert notice header:\n%s", out)
	}
}

// TestGen_CertificatesDisabled_SkipsMkdirAndNotice verifies that when
// the workspace omits [certificates] (or sets enable=false), `cocoon
// gen` neither creates ~/.cocoon/certs on the host nor emits the TLS
// notice block. This is the opt-out invariant: cert-free teams get
// no host-side side effects and no cert wiring in their stdout.
func TestGen_CertificatesDisabled_SkipsMkdirAndNotice(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(minimalWorkspaceTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen: %v\nstderr:\n%s", err, stderr.String())
	}

	certsDir := filepath.Join(home, ".cocoon", "certs")
	if _, statErr := os.Stat(certsDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("certs dir should NOT exist when [certificates] is absent: stat err = %v", statErr)
	}

	out := stdout.String()
	for _, mustNot := range []string{
		"created host directory",
		"Host TLS certificates:",
		"~/.cocoon/certs",
	} {
		if strings.Contains(out, mustNot) {
			t.Errorf("disabled-cert stdout should not contain %q\n--- got ---\n%s", mustNot, out)
		}
	}
}

// TestGen_NoDevcontainerSuppressesVSCodeHint covers the
// `[workspace] devcontainer = false` branch: devcontainer.json is not
// written and the VS Code line is suppressed, but the docker compose
// hint still appears.
func TestGen_NoDevcontainerSuppressesVSCodeHint(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	body := strings.Replace(minimalWorkspaceTOML, "devcontainer = true", "devcontainer = false", 1)
	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen: %v\nstderr:\n%s", err, stderr.String())
	}

	if _, statErr := os.Stat(filepath.Join(work, ".devcontainer/devcontainer.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("devcontainer.json should not exist when devcontainer=false: %v", statErr)
	}

	out := stdout.String()
	if !strings.Contains(out, "docker compose -f .devcontainer/docker-compose.yml up -d") {
		t.Errorf("docker compose hint missing in stdout:\n%s", out)
	}
	if strings.Contains(out, "Reopen in Container") {
		t.Errorf("VS Code line should be suppressed when devcontainer=false:\n%s", out)
	}
}

// TestGen_ExplicitWorkspaceAndOutput checks that --workspace and
// --output redirect both inputs and outputs without relying on cwd
// discovery.
func TestGen_ExplicitWorkspaceAndOutput(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)

	wsPath := filepath.Join(work, "custom-workspace.toml")
	if err := os.WriteFile(wsPath, []byte(minimalWorkspaceTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}
	outDir := filepath.Join(work, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir out: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"--workspace", wsPath, "--output", outDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen: %v\nstderr:\n%s", err, stderr.String())
	}

	for _, rel := range []string{"Dockerfile", "docker-compose.yml", "devcontainer.json"} {
		path := filepath.Join(outDir, ".devcontainer", rel)
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("expected %s to exist: %v", path, statErr)
		}
	}
}

// homeFilesWorkspaceTOML is `minimalWorkspaceTOML` plus a [home_files]
// section listing a top-level file and a nested file, so gen exercises
// both the no-parent and parent-mkdir branches in ensureHomeFiles.
const homeFilesWorkspaceTOML = minimalWorkspaceTOML + `
[home_files]
files = [".claude.json", ".gemini/oauth_creds.json"]
`

// TestGen_HomeFiles_TouchesAndPrintsNotice verifies that `cocoon gen`
// touches each [home_files] entry on the host with mode 0o600 (creating
// parent dirs as needed), surfaces the per-file "created" line on the
// first run only, and prints the host-files notice both runs.
func TestGen_HomeFiles_TouchesAndPrintsNotice(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(homeFilesWorkspaceTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen #1: %v\nstderr:\n%s", err, stderr.String())
	}

	for _, rel := range []string{".claude.json", ".gemini/oauth_creds.json"} {
		path := filepath.Join(home, rel)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected %s after gen: %v", path, err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("%s mode = %o, want 0600", path, mode)
		}
		if info.IsDir() {
			t.Errorf("%s should be a regular file, not a directory", path)
		}
	}

	out := stdout.String()
	for _, want := range []string{
		"created " + filepath.Join(home, ".claude.json"),
		"created " + filepath.Join(home, ".gemini/oauth_creds.json"),
		"Host files for [home_files]:",
		"Verify these files exist on the host",
		"~/.claude.json",
		"~/.gemini/oauth_creds.json",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("first-run stdout missing %q\n--- got ---\n%s", want, out)
		}
	}

	stdout.Reset()
	stderr.Reset()
	cmd2 := gencli.NewCommand(&stdout, &stderr)
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("gen #2: %v\nstderr:\n%s", err, stderr.String())
	}
	out = stdout.String()
	if strings.Contains(out, "created "+filepath.Join(home, ".claude.json")) {
		t.Errorf("re-run should not announce file creation:\n%s", out)
	}
	if !strings.Contains(out, "Host files for [home_files]:") {
		t.Errorf("re-run should still emit the home_files notice header:\n%s", out)
	}
}

// TestGen_HomeFiles_PreservesExistingFileMode verifies that an existing
// home_files entry with mode 0644 (e.g. user wrote it themselves) is left
// untouched on subsequent gen runs. ensureHomeFiles must be idempotent
// against existing files.
func TestGen_HomeFiles_PreservesExistingFileMode(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	target := filepath.Join(home, ".claude.json")
	// 0644 is deliberate: the test pins that ensureHomeFiles leaves the
	// mode of an existing file untouched (i.e. does not narrow it to 0600).
	if err := os.WriteFile(target, []byte(`{"keep":"me"}`), 0o644); err != nil { //nolint:gosec // see comment above
		t.Fatalf("seed existing file: %v", err)
	}

	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(homeFilesWorkspaceTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen: %v\nstderr:\n%s", err, stderr.String())
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat %s: %v", target, err)
	}
	if mode := info.Mode().Perm(); mode != 0o644 {
		t.Errorf("existing file mode = %o, want 0644 (unchanged)", mode)
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}
	if string(body) != `{"keep":"me"}` {
		t.Errorf("existing file body was modified: %q", body)
	}
	if strings.Contains(stdout.String(), "created "+target) {
		t.Errorf("gen should not announce creation of an existing file:\n%s", stdout.String())
	}
}

// TestGen_HomeFiles_RejectsExistingDirectory verifies that when a
// home_files entry already exists as a directory on the host (typically
// because a prior `docker compose up` ran while the file was missing and
// Docker auto-created it), `cocoon gen` fails with ErrFailure and the
// stderr message points the user at the `rm -rf` recovery path.
func TestGen_HomeFiles_RejectsExistingDirectory(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	target := filepath.Join(home, ".claude.json")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("seed directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(homeFilesWorkspaceTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected gen to fail when home_files entry is a directory")
	}
	if !errors.Is(err, clihelpers.ErrFailure) {
		t.Errorf("expected ErrFailure, got %v", err)
	}
	if !strings.Contains(stderr.String(), "rm -rf "+target) {
		t.Errorf("stderr should suggest `rm -rf %s`:\n%s", target, stderr.String())
	}
}

// TestGen_HomeFilesAbsent_NoNotice verifies the opt-out invariant: a
// workspace without [home_files] gets no host-files notice and no
// host-side file creation.
func TestGen_HomeFilesAbsent_NoNotice(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(minimalWorkspaceTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen: %v\nstderr:\n%s", err, stderr.String())
	}

	out := stdout.String()
	for _, mustNot := range []string{
		"Host files for [home_files]:",
		"Verify these files exist on the host",
	} {
		if strings.Contains(out, mustNot) {
			t.Errorf("home_files-absent stdout should not contain %q\n--- got ---\n%s", mustNot, out)
		}
	}
}

// dockerCLINoSocketWorkspaceTOML enables the docker-cli plugin but leaves
// docker_socket unset — the misconfiguration warnDockerCLIWithoutSocket flags.
const dockerCLINoSocketWorkspaceTOML = `[workspace]
mount_root = "."
devcontainer = true

[container]
service_name = "demo"
username = "alice"
image = "ubuntu"
image_version = "24.04"

[plugins]
enable = ["docker-cli"]

[apt]
packages = []
`

// runGenForWarn runs `cocoon gen` over body in an isolated tempdir and
// returns captured stderr. It fails the test if gen itself errors.
func runGenForWarn(t *testing.T, body string) string {
	t.Helper()
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}
	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen: %v\nstderr:\n%s", err, stderr.String())
	}
	return stderr.String()
}

const dockerCLIWarnSubstr = "docker-cli plugin is enabled"

// TestGen_DockerCLIWithoutSocket_Warns asserts gen flags the docker-cli
// plugin enabled without docker_socket — the in-container client has no
// daemon to reach.
//
//nolint:paralleltest // runGenForWarn uses t.Setenv + t.Chdir
func TestGen_DockerCLIWithoutSocket_Warns(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	stderr := runGenForWarn(t, dockerCLINoSocketWorkspaceTOML)
	if !strings.Contains(stderr, dockerCLIWarnSubstr) {
		t.Errorf("expected docker_socket warning on stderr\n--- got ---\n%s", stderr)
	}
}

// TestGen_DockerCLIWithSocket_NoWarn asserts the warning stays silent once
// docker_socket = true is set alongside the docker-cli plugin.
//
//nolint:paralleltest // runGenForWarn uses t.Setenv + t.Chdir
func TestGen_DockerCLIWithSocket_NoWarn(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	body := strings.Replace(dockerCLINoSocketWorkspaceTOML,
		`image_version = "24.04"`,
		"image_version = \"24.04\"\ndocker_socket = true", 1)
	stderr := runGenForWarn(t, body)
	if strings.Contains(stderr, dockerCLIWarnSubstr) {
		t.Errorf("docker_socket warning should be silent when docker_socket = true\n--- got ---\n%s", stderr)
	}
}

// TestGen_NoDockerCLI_NoWarn asserts the warning does not fire when the
// docker-cli plugin is absent, even without docker_socket.
//
//nolint:paralleltest // runGenForWarn uses t.Setenv + t.Chdir
func TestGen_NoDockerCLI_NoWarn(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	stderr := runGenForWarn(t, minimalWorkspaceTOML)
	if strings.Contains(stderr, dockerCLIWarnSubstr) {
		t.Errorf("docker_socket warning should not fire without docker-cli\n--- got ---\n%s", stderr)
	}
}

// TestGen_MissingWorkspaceReturnsUsage covers the discovery-miss path
// when workspace.toml is absent and no --workspace flag is given.
func TestGen_MissingWorkspaceReturnsUsage(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected ErrUsage, got nil")
	}
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("expected ErrUsage, got %v", err)
	}
}
