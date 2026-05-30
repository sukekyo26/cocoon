package generate_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/generate"
)

// manage.sh drives `docker` against the host daemon, which a unit test must
// not touch. These tests mirror dockerfile/entrypoint_behavior_test.go: the
// embedded script runs as a subprocess with PATH pointing only at a stub
// directory, so `docker` is a logging shim under our control. The docker stub
// appends "docker\t<args>" to $STUB_LOG; assertions read that log to pin which
// compose commands run, with which flags. dirname/grep/cut/cat are passed
// through to the real binaries (manage.sh needs them for SCRIPT_DIR resolution,
// project-name lookup, and usage output).
//
// What only e2e (e2e/docker-roundtrip.sh) can prove — that the real docker
// compose actually tears down / rebuilds the project — is left to e2e. These
// unit tests pin the branching: which command each subcommand maps to, the
// fail-closed guards (no docker / no buildx / missing files), and that every
// invocation stays scoped to the generated compose + env file.

type manageRun struct {
	exitCode int
	stdout   string
	stderr   string
	log      string
	projDir  string
}

type manageOpts struct {
	args          []string
	omitDocker    bool // drop the docker stub so `command -v docker` fails
	omitProjFiles bool // skip writing docker-compose.yml / .env (preflight guard)
	buildxExit    int  // exit code of the `docker buildx version` probe (0 = present)
}

// runManage writes manage.sh next to a stub compose/env pair, stubs PATH, and
// runs `bash manage.sh <args>` as a subprocess.
func runManage(t *testing.T, o manageOpts) manageRun {
	t.Helper()
	lookOrSkip := func(name string) string {
		p, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s not on PATH", name)
		}
		return p
	}
	bashPath := lookOrSkip("bash")

	stubDir := t.TempDir()
	projDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "calls.log")

	// docker logs every call, honours STUB_BUILDX_EXIT for the buildx probe,
	// and otherwise succeeds. dirname/grep/cut/cat delegate to the real binary.
	passthrough := func(name string) string { return "exec " + lookOrSkip(name) + " \"$@\"" }
	stubs := map[string]string{
		"dirname": passthrough("dirname"),
		"grep":    passthrough("grep"),
		"cut":     passthrough("cut"),
		"cat":     passthrough("cat"),
	}
	if !o.omitDocker {
		stubs["docker"] = "printf 'docker\\t%s\\n' \"$*\" >> \"$STUB_LOG\"\n" +
			"if [ \"$1\" = buildx ] && [ \"$2\" = version ]; then exit \"${STUB_BUILDX_EXIT:-0}\"; fi\n" +
			"exit 0"
	}
	for name, body := range stubs {
		script := "#!/bin/sh\n" + body + "\n"
		if writeErr := os.WriteFile(filepath.Join(stubDir, name), []byte(script), 0o700); writeErr != nil { //nolint:gosec // test stub must be executable
			t.Fatalf("write stub %s: %v", name, writeErr)
		}
	}

	// manage.sh self-locates SCRIPT_DIR and derives COMPOSE_FILE / ENV_FILE
	// next to itself; preflight requires both to exist.
	managePath := filepath.Join(projDir, "manage.sh")
	if writeErr := os.WriteFile(managePath, []byte(generate.ManageScript()), 0o700); writeErr != nil { //nolint:gosec // generated script is executable
		t.Fatalf("write manage.sh: %v", writeErr)
	}
	if !o.omitProjFiles {
		if writeErr := os.WriteFile(filepath.Join(projDir, "docker-compose.yml"), []byte("services: {}\n"), 0o600); writeErr != nil {
			t.Fatalf("write compose: %v", writeErr)
		}
		if writeErr := os.WriteFile(filepath.Join(projDir, ".env"), []byte("COMPOSE_PROJECT_NAME=testproj\n"), 0o600); writeErr != nil {
			t.Fatalf("write env: %v", writeErr)
		}
	}

	cmd := exec.CommandContext(t.Context(), bashPath, append([]string{managePath}, o.args...)...)
	cmd.Env = []string{"PATH=" + stubDir, "STUB_LOG=" + logPath}
	if o.buildxExit != 0 {
		cmd.Env = append(cmd.Env, "STUB_BUILDX_EXIT="+strconv.Itoa(o.buildxExit))
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	exitCode := 0
	if runErr := cmd.Run(); runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			t.Fatalf("run manage.sh: %v\nstderr: %s", runErr, stderr.String())
		}
		exitCode = exitErr.ExitCode()
	}
	logBytes, _ := os.ReadFile(logPath) //nolint:errcheck // absent log => empty string is a valid assertion target
	return manageRun{
		exitCode: exitCode,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		log:      string(logBytes),
		projDir:  projDir,
	}
}

// logHasDocker reports whether any logged docker invocation contains argSub.
// Every stub call is `docker`, so the command name is implicit.
func logHasDocker(log, argSub string) bool {
	for _, line := range strings.Split(log, "\n") {
		_, args, ok := strings.Cut(line, "\t")
		if ok && strings.Contains(args, argSub) {
			return true
		}
	}
	return false
}

// TestManage_CleanAllRemovesEverything pins that bare `clean` tears down
// containers, networks, volumes and the locally-built image.
//
//nolint:paralleltest // subprocess + PATH stubs, kept serial for clarity
func TestManage_CleanAllRemovesEverything(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{"-y", "clean"}})
	if r.exitCode != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s\nlog:\n%s", r.exitCode, r.stderr, r.log)
	}
	if !logHasDocker(r.log, "down --volumes --rmi local --remove-orphans") {
		t.Errorf("clean did not run `compose down --volumes --rmi local --remove-orphans`\nlog:\n%s", r.log)
	}
}

// TestManage_CleanContainersOnly pins that `clean containers` removes only
// containers (rm --stop --force), leaving networks/volumes/image.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestManage_CleanContainersOnly(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{"-y", "clean", "containers"}})
	if r.exitCode != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", r.exitCode, r.stderr)
	}
	if !logHasDocker(r.log, "rm --stop --force") {
		t.Errorf("clean containers did not run `compose rm --stop --force`\nlog:\n%s", r.log)
	}
	if logHasDocker(r.log, "down") {
		t.Errorf("clean containers must not run `down`\nlog:\n%s", r.log)
	}
}

// TestManage_CleanImageKeepsVolumes pins that `clean image` removes the image
// but never passes --volumes (user data is kept).
//
//nolint:paralleltest // subprocess + PATH stubs
func TestManage_CleanImageKeepsVolumes(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{"-y", "clean", "image"}})
	if r.exitCode != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", r.exitCode, r.stderr)
	}
	if !logHasDocker(r.log, "down --rmi local --remove-orphans") {
		t.Errorf("clean image did not run `compose down --rmi local --remove-orphans`\nlog:\n%s", r.log)
	}
	if logHasDocker(r.log, "--volumes") {
		t.Errorf("clean image must NOT pass --volumes (data is kept)\nlog:\n%s", r.log)
	}
}

// TestManage_CleanVolumesKeepsImage pins that `clean volumes` removes volumes
// but never passes --rmi (the locally-built image is kept for fast rebuild).
//
//nolint:paralleltest // subprocess + PATH stubs
func TestManage_CleanVolumesKeepsImage(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{"-y", "clean", "volumes"}})
	if r.exitCode != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", r.exitCode, r.stderr)
	}
	if !logHasDocker(r.log, "down --volumes --remove-orphans") {
		t.Errorf("clean volumes did not run `compose down --volumes --remove-orphans`\nlog:\n%s", r.log)
	}
	if logHasDocker(r.log, "--rmi") {
		t.Errorf("clean volumes must NOT pass --rmi (image is kept)\nlog:\n%s", r.log)
	}
}

// TestManage_RebuildBuildsNoCacheAndRecreates pins that `rebuild` runs a
// no-cache build followed by a forced recreate.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestManage_RebuildBuildsNoCacheAndRecreates(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{"-y", "rebuild"}})
	if r.exitCode != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s\nlog:\n%s", r.exitCode, r.stderr, r.log)
	}
	if !logHasDocker(r.log, "build --no-cache") {
		t.Errorf("rebuild did not run `compose build --no-cache`\nlog:\n%s", r.log)
	}
	if !logHasDocker(r.log, "up -d --force-recreate") {
		t.Errorf("rebuild did not run `compose up -d --force-recreate`\nlog:\n%s", r.log)
	}
}

// TestManage_RebuildFailsClosedWithoutBuildx pins the fail-closed guard: when
// `docker buildx version` fails, rebuild must exit non-zero before building
// rather than fall back to the legacy builder.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestManage_RebuildFailsClosedWithoutBuildx(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{"-y", "rebuild"}, buildxExit: 1})
	if r.exitCode != 1 {
		t.Fatalf("exit = %d, want 1 (missing buildx must fail closed)\nstderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stderr, "docker buildx is not installed") {
		t.Errorf("stderr = %q, want a 'docker buildx is not installed' message", r.stderr)
	}
	if logHasDocker(r.log, "build --no-cache") {
		t.Errorf("rebuild ran the build despite the missing-buildx guard\nlog:\n%s", r.log)
	}
}

// TestManage_PruneCacheTargetsGlobalBuilder pins that `prune-cache` runs the
// GLOBAL `docker builder prune -f` and warns it is host-wide.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestManage_PruneCacheTargetsGlobalBuilder(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{"-y", "prune-cache"}})
	if r.exitCode != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", r.exitCode, r.stderr)
	}
	if !logHasDocker(r.log, "builder prune -f") {
		t.Errorf("prune-cache did not run `docker builder prune -f`\nlog:\n%s", r.log)
	}
	if !strings.Contains(r.stderr, "GLOBAL") {
		t.Errorf("prune-cache must warn the prune is GLOBAL\nstderr: %s", r.stderr)
	}
}

// TestManage_ComposeScopedToGeneratedFiles pins that every compose invocation
// is scoped to this project via -f <compose> --env-file <env>.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestManage_ComposeScopedToGeneratedFiles(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{"-y", "clean"}})
	if r.exitCode != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", r.exitCode, r.stderr)
	}
	want := "-f " + filepath.Join(r.projDir, "docker-compose.yml") + " --env-file " + filepath.Join(r.projDir, ".env")
	if !logHasDocker(r.log, want) {
		t.Errorf("compose call not scoped to the generated files\nwant substring: %s\nlog:\n%s", want, r.log)
	}
}

// TestManage_PreflightFailsWithoutDocker pins the preflight guard: with no
// docker on PATH the script dies before touching anything.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestManage_PreflightFailsWithoutDocker(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{"-y", "clean"}, omitDocker: true})
	if r.exitCode != 1 {
		t.Fatalf("exit = %d, want 1 (missing docker must fail closed)\nstderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stderr, "docker not found on PATH") {
		t.Errorf("stderr = %q, want 'docker not found on PATH'", r.stderr)
	}
}

// TestManage_PreflightFailsWithoutComposeFile pins that a missing generated
// compose/env pair fails with guidance to run `cocoon gen`.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestManage_PreflightFailsWithoutComposeFile(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{"-y", "clean"}, omitProjFiles: true})
	if r.exitCode != 1 {
		t.Fatalf("exit = %d, want 1 (missing compose file must fail)\nstderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stderr, "run 'cocoon gen' first") {
		t.Errorf("stderr = %q, want it to point at `cocoon gen`", r.stderr)
	}
}

// TestManage_ConfirmRequiresTerminalOrYes pins that a destructive command
// without -y refuses to proceed when stdin is not a terminal.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestManage_ConfirmRequiresTerminalOrYes(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{"clean"}}) // no -y, stdin is not a tty
	if r.exitCode != 1 {
		t.Fatalf("exit = %d, want 1 (no -y + no tty must abort)\nstderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stderr, "no terminal for confirmation") {
		t.Errorf("stderr = %q, want 'no terminal for confirmation'", r.stderr)
	}
	if logHasDocker(r.log, "down") {
		t.Errorf("destructive command ran without confirmation\nlog:\n%s", r.log)
	}
}

// TestManage_UnknownCommandExitsUsage pins that an unknown command exits 2.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestManage_UnknownCommandExitsUsage(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{"-y", "bogus"}})
	if r.exitCode != 2 {
		t.Fatalf("exit = %d, want 2\nstderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stderr, "unknown command") {
		t.Errorf("stderr = %q, want 'unknown command'", r.stderr)
	}
}

// TestManage_NoArgsExitsUsage pins that invoking with no command exits 2 with
// usage on stderr.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestManage_NoArgsExitsUsage(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{}})
	if r.exitCode != 2 {
		t.Fatalf("exit = %d, want 2\nstderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stderr, "Usage:") {
		t.Errorf("stderr = %q, want usage text", r.stderr)
	}
}

// TestManage_HelpFlagExitsZero pins that -h prints usage to stdout and exits 0.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestManage_HelpFlagExitsZero(t *testing.T) {
	r := runManage(t, manageOpts{args: []string{"-h"}})
	if r.exitCode != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stdout, "Usage:") {
		t.Errorf("stdout = %q, want usage text", r.stdout)
	}
}
