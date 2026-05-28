package dockerfile_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/generate/dockerfile"
)

// The entrypoint runs as root and calls usermod / groupmod / setpriv etc.,
// none of which work (or are safe) in a unit-test process. These tests run
// the embedded script as a subprocess with PATH pointing only at a stub
// directory, so every external command is a logging shim under our control.
// Each stub appends "<name>\t<args>" to $STUB_LOG; assertions read that log.
//
// What only e2e (e2e/docker-roundtrip.sh) can prove — that the real
// usermod/setpriv produce the expected uid/gid and privilege drop — is left
// to e2e. These unit tests pin the branching: which commands run, with which
// arguments, under which inputs.
//
// Note: `[ -S /var/run/docker.sock ]` reads a hardcoded host path we cannot
// control, so the docker-socket branch may or may not fire depending on the
// host. Assertions therefore key on remap-specific argument shapes
// (`usermod -o -u`, `groupmod -o -g`) rather than bare command presence.

type entrypointRun struct {
	exitCode int
	stderr   string
	log      string
}

// runEntrypoint executes the embedded entrypoint with a stubbed PATH.
// env supplies HOST_UID/HOST_GID/COCOON_BIND_PATHS etc. omitSetpriv drops
// the setpriv stub so `command -v setpriv` fails.
func runEntrypoint(t *testing.T, env map[string]string, omitSetpriv bool) entrypointRun {
	t.Helper()
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not on PATH")
	}
	cutPath, err := exec.LookPath("cut")
	if err != nil {
		t.Skip("cut not on PATH")
	}

	stubDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "calls.log")

	// bodies run after the per-stub log line. id reports root for the bare
	// `id -u` (so the script takes the root path) and 1000 for the user
	// lookups; getent yields a passwd line so $user_home resolves; cut
	// delegates to the real binary so `getent | cut` works under PATH=stubDir.
	bodies := map[string]string{
		"id": "if [ \"$1\" = \"-u\" ] && [ -z \"$2\" ]; then echo 0; exit 0; fi\n" +
			"if [ \"$1\" = \"-u\" ]; then echo 1000; exit 0; fi\n" +
			"if [ \"$1\" = \"-g\" ]; then echo 1000; exit 0; fi\necho 0",
		"getent":   "if [ \"$1\" = \"passwd\" ]; then echo \"developer:x:1000:1000::/home/developer:/bin/bash\"; fi",
		"stat":     "echo 1000",
		"cut":      "exec " + cutPath + " \"$@\"",
		"usermod":  "",
		"groupmod": "",
		"groupadd": "",
		"chown":    "",
		"find":     "",
		"setpriv":  "",
	}
	for name, body := range bodies {
		if name == "setpriv" && omitSetpriv {
			continue
		}
		script := "#!/bin/sh\nprintf '%s\\t%s\\n' '" + name + "' \"$*\" >> \"$STUB_LOG\"\n"
		if body != "" {
			script += body + "\n"
		}
		script += "exit 0\n"
		if writeErr := os.WriteFile(filepath.Join(stubDir, name), []byte(script), 0o700); writeErr != nil { //nolint:gosec // test stub must be executable
			t.Fatalf("write stub %s: %v", name, writeErr)
		}
	}

	scriptPath := filepath.Join(t.TempDir(), "docker-entrypoint.sh")
	if writeErr := os.WriteFile(scriptPath, []byte(dockerfile.EntrypointScript()), 0o600); writeErr != nil {
		t.Fatalf("write entrypoint: %v", writeErr)
	}

	cmd := exec.CommandContext(t.Context(), bashPath, scriptPath, "/bin/true")
	cmd.Env = []string{
		"PATH=" + stubDir,
		"STUB_LOG=" + logPath,
		"HOME=/home/developer",
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr

	exitCode := 0
	if runErr := cmd.Run(); runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			t.Fatalf("run entrypoint: %v\nstderr: %s", runErr, stderr.String())
		}
		exitCode = exitErr.ExitCode()
	}
	logBytes, _ := os.ReadFile(logPath) //nolint:errcheck // absent log => empty string is a valid assertion target
	return entrypointRun{exitCode: exitCode, stderr: stderr.String(), log: string(logBytes)}
}

// logHasArgs reports whether any logged invocation of cmd contains argSub.
func logHasArgs(log, cmd, argSub string) bool {
	for _, line := range strings.Split(log, "\n") {
		name, args, ok := strings.Cut(line, "\t")
		if ok && name == cmd && strings.Contains(args, argSub) {
			return true
		}
	}
	return false
}

// logHasCmd reports whether cmd was invoked at all.
func logHasCmd(log, cmd string) bool {
	for _, line := range strings.Split(log, "\n") {
		if name, _, ok := strings.Cut(line, "\t"); ok && name == cmd {
			return true
		}
	}
	return false
}

// TestEntrypoint_RemapAppliesUserAndGroup pins that when the target uid/gid
// (from HOST_UID/HOST_GID) differ from the build-time ids, the script remaps
// both with the non-unique `-o` flag, sweeps ownership via find, and drops
// privileges via setpriv.
//
//nolint:paralleltest // subprocess + PATH stubs, kept serial for clarity
func TestEntrypoint_RemapAppliesUserAndGroup(t *testing.T) {
	r := runEntrypoint(t, map[string]string{"HOST_UID": "1500", "HOST_GID": "1600"}, false)
	if r.exitCode != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s\nlog:\n%s", r.exitCode, r.stderr, r.log)
	}
	if !logHasArgs(r.log, "groupmod", "-o -g 1600 developer") {
		t.Errorf("groupmod not called with the remapped gid\nlog:\n%s", r.log)
	}
	if !logHasArgs(r.log, "usermod", "-o -u 1500 developer") {
		t.Errorf("usermod not called with the remapped uid\nlog:\n%s", r.log)
	}
	if !logHasCmd(r.log, "find") {
		t.Errorf("ownership sweep (find) not run after remap\nlog:\n%s", r.log)
	}
	if !logHasCmd(r.log, "setpriv") {
		t.Errorf("setpriv (privilege drop) not invoked\nlog:\n%s", r.log)
	}
}

// TestEntrypoint_RootlessGuardSkipsRemap pins the rootless / Docker-Desktop
// guard: a target uid or gid of 0 (workspace owned by root) must NOT remap
// the user to uid 0, which would alias it to root.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestEntrypoint_RootlessGuardSkipsRemap(t *testing.T) {
	r := runEntrypoint(t, map[string]string{"HOST_UID": "0", "HOST_GID": "0"}, false)
	if r.exitCode != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", r.exitCode, r.stderr)
	}
	if logHasCmd(r.log, "groupmod") {
		t.Errorf("groupmod ran despite the uid/gid-0 rootless guard\nlog:\n%s", r.log)
	}
	if logHasArgs(r.log, "usermod", "-o -u") {
		t.Errorf("usermod remap ran despite the uid/gid-0 rootless guard\nlog:\n%s", r.log)
	}
	if logHasCmd(r.log, "find") {
		t.Errorf("ownership sweep ran despite no remap\nlog:\n%s", r.log)
	}
}

// TestEntrypoint_NoRemapWhenIdsMatch pins the no-op guard: when the target
// ids already equal the build-time ids, neither usermod/groupmod nor the
// chown sweep run, but privileges are still dropped.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestEntrypoint_NoRemapWhenIdsMatch(t *testing.T) {
	r := runEntrypoint(t, map[string]string{"HOST_UID": "1000", "HOST_GID": "1000"}, false)
	if r.exitCode != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", r.exitCode, r.stderr)
	}
	if logHasCmd(r.log, "groupmod") || logHasArgs(r.log, "usermod", "-o -u") {
		t.Errorf("remap ran even though target ids match build-time ids\nlog:\n%s", r.log)
	}
	if logHasCmd(r.log, "find") {
		t.Errorf("ownership sweep ran even though ids match\nlog:\n%s", r.log)
	}
	if !logHasCmd(r.log, "setpriv") {
		t.Errorf("setpriv must still run on the no-remap path\nlog:\n%s", r.log)
	}
}

// TestEntrypoint_ChownSweepPrunesBindPaths pins that the chown sweep builds
// a find prune expression from COCOON_BIND_PATHS so bind-mounted host paths
// are not re-owned.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestEntrypoint_ChownSweepPrunesBindPaths(t *testing.T) {
	r := runEntrypoint(t, map[string]string{
		"HOST_UID":          "1500",
		"HOST_GID":          "1500",
		"COCOON_BIND_PATHS": "/foo:/bar",
	}, false)
	if r.exitCode != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", r.exitCode, r.stderr)
	}
	if !logHasArgs(r.log, "find", "-path /foo -prune -o -path /bar -prune -o") {
		t.Errorf("find prune expression not built from COCOON_BIND_PATHS\nlog:\n%s", r.log)
	}
}

// TestEntrypoint_SetprivMissingFailsClosed pins the fail-closed guard: if
// setpriv is absent the script must exit non-zero rather than silently
// continuing as root.
//
//nolint:paralleltest // subprocess + PATH stubs
func TestEntrypoint_SetprivMissingFailsClosed(t *testing.T) {
	r := runEntrypoint(t, map[string]string{"HOST_UID": "1000", "HOST_GID": "1000"}, true)
	if r.exitCode != 1 {
		t.Fatalf("exit = %d, want 1 (setpriv missing must fail closed)\nstderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stderr, "setpriv not found") {
		t.Errorf("stderr = %q, want a 'setpriv not found' message", r.stderr)
	}
}
