package plugincli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	plugincli "github.com/sukekyo26/cocoon/internal/cli/plugin"
)

func runCmd(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func assertGoldenDir(t *testing.T, gotDir, wantDir string, files []string) {
	t.Helper()
	for _, name := range files {
		gotPath := filepath.Join(gotDir, name)
		wantPath := filepath.Join(wantDir, name)
		got := mustRead(t, gotPath)
		want := mustRead(t, wantPath)
		if got != want {
			t.Errorf("%s mismatch (golden %s)\n--- got ---\n%s\n--- want ---\n%s",
				name, wantPath, got, want)
		}
	}
}

func TestScaffoldGoldenCurlPipe(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stdout, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--template", "curl-pipe",
		"--version-capable",
		"--name", "Demo",
		"--description", "Demo plugin (https://example.com)",
		"--non-interactive",
	)
	if err != nil {
		t.Fatalf("Run err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "OK:") {
		t.Errorf("stdout missing OK marker: %q", stdout)
	}
	assertGoldenDir(t, filepath.Join(dir, "demo"),
		"testdata/scaffold-curl-pipe",
		[]string{"plugin.toml", "install.sh"})
	assertExecutable(t, filepath.Join(dir, "demo", "install.sh"))
}

func TestScaffoldGoldenTarball(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--template", "tarball",
		"--version-capable",
		"--requires-root",
		"--with-install-user",
		"--name", "Demo Tar",
		"--description", "Demo tarball plugin (https://example.com/tar)",
		"--non-interactive",
	)
	if err != nil {
		t.Fatalf("Run err=%v stderr=%s", err, stderr)
	}
	assertGoldenDir(t, filepath.Join(dir, "demo"),
		"testdata/scaffold-tarball",
		[]string{"plugin.toml", "install.sh", "install_user.sh"})
	assertExecutable(t, filepath.Join(dir, "demo", "install.sh"))
	assertExecutable(t, filepath.Join(dir, "demo", "install_user.sh"))
}

func TestScaffoldGoldenGeneric(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--template", "generic",
		"--name", "Demo Generic",
		"--description", "Demo generic plugin (https://example.com/g)",
		"--non-interactive",
	)
	if err != nil {
		t.Fatalf("Run err=%v stderr=%s", err, stderr)
	}
	assertGoldenDir(t, filepath.Join(dir, "demo"),
		"testdata/scaffold-generic",
		[]string{"plugin.toml", "install.sh"})
}

func TestScaffoldRejectsInvalidID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, err := runCmd(t,
		"scaffold", "Bad_ID",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "x",
		"--description", "x (https://x)",
	)
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(stderr, "Invalid plugin id") &&
		!strings.Contains(stderr, "プラグイン ID") {
		t.Errorf("stderr does not mention id rule: %q", stderr)
	}
}

// When --plugins-dir is omitted and no workspace.toml is discoverable from
// cwd, scaffold must surface an actionable error instead of silently writing
// to ./plugins/<id>/. Regression guard for the v0.1.0 default that left
// stray plugins directories at the cocoon repo root.
//
//nolint:paralleltest // t.Chdir mutates process-wide state; cannot run in parallel.
func TestScaffoldRequiresPluginsDirOrWorkspace(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	_, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--non-interactive",
		"--name", "Demo",
		"--description", "Demo (https://example.com)",
	)
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(stderr, "--plugins-dir") {
		t.Errorf("stderr should mention --plugins-dir: %q", stderr)
	}
}

func TestScaffoldRequiresIDArg(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, err := runCmd(t,
		"scaffold",
		"--plugins-dir", dir,
		"--non-interactive",
	)
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if stderr == "" {
		t.Errorf("expected non-empty stderr")
	}
}

func TestScaffoldRequiresForceOnExistingDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "demo"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "Demo",
		"--description", "Demo (https://example.com)",
	)
	if !errors.Is(err, plugincli.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("stderr should mention --force: %q", stderr)
	}
}

func TestScaffoldOverwritesWithForce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "demo")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stale := filepath.Join(target, "plugin.toml")
	if err := os.WriteFile(stale, []byte("# stale\n"), 0o600); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	_, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--force",
		"--name", "Demo",
		"--description", "Demo (https://example.com)",
	)
	if err != nil {
		t.Fatalf("Run err=%v stderr=%s", err, stderr)
	}
	body := mustRead(t, stale)
	if strings.Contains(body, "# stale") {
		t.Errorf("plugin.toml was not overwritten: %q", body)
	}
}

func TestScaffoldRejectsTarballWithoutVersionCapable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--template", "tarball",
		"--non-interactive",
		"--name", "Demo",
		"--description", "Demo (https://example.com)",
	)
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(stderr, "tarball") {
		t.Errorf("stderr should mention tarball: %q", stderr)
	}
}

func TestScaffoldRejectsUnknownTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, _, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--template", "wat",
		"--non-interactive",
		"--name", "Demo",
		"--description", "Demo (https://example.com)",
	)
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestScaffoldRequiresName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--description", "Demo (https://example.com)",
	)
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(stderr, "name") {
		t.Errorf("stderr should mention name flag: %q", stderr)
	}
}

func TestScaffoldHelp(t *testing.T) {
	t.Parallel()
	stdout, _, err := runCmd(t, "scaffold", "--help")
	if err != nil {
		t.Fatalf("Run err=%v", err)
	}
	if !strings.Contains(stdout, "cocoon plugin scaffold") {
		t.Errorf("help missing banner: %q", stdout)
	}
}

func TestPluginUsage(t *testing.T) {
	t.Parallel()
	stdout, _, err := runCmd(t, "help")
	if err != nil {
		t.Fatalf("Run err=%v", err)
	}
	if !strings.Contains(stdout, "scaffold") {
		t.Errorf("usage missing scaffold subcommand: %q", stdout)
	}
}

func TestPluginUnknownSubcommand(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd(t, "frobnicate")
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "unknown subcommand") {
		t.Errorf("err missing 'unknown subcommand': %v", err)
	}
}

// TestScaffoldRejectsExistingFileTarget exercises the dirExists branch where
// the path exists but is not a directory.
func TestScaffoldRejectsExistingFileTarget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Pre-create a file at the would-be plugin directory.
	conflict := filepath.Join(dir, "demo")
	if err := os.WriteFile(conflict, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "Demo",
		"--description", "Demo (https://example.com)",
		"--template", "generic",
	)
	if !errors.Is(err, plugincli.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
	if !strings.Contains(stderr, "demo") {
		t.Errorf("expected stderr to mention path: %q", stderr)
	}
}

// TestScaffoldDescriptionMissing exercises the description+URL validator path.
func TestScaffoldDescriptionMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, _, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "Demo",
		// no --description
	)
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

// TestScaffoldNonInteractiveRejectsWhitespaceName asserts that a whitespace-only
// --name is rejected with ErrUsage in non-interactive mode (i.e. validateNameInput
// is applied symmetrically with the interactive form).
func TestScaffoldNonInteractiveRejectsWhitespaceName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "   ",
		"--description", "Demo (https://example.com)",
	)
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(stderr, "--name") {
		t.Errorf("stderr should mention --name: %q", stderr)
	}
}

// TestScaffoldNonInteractiveRejectsURLLessDescription asserts that a --description
// without an embedded URL is rejected in non-interactive mode.
func TestScaffoldNonInteractiveRejectsURLLessDescription(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "Demo",
		"--description", "Demo without URL",
	)
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(stderr, "--description") {
		t.Errorf("stderr should mention --description: %q", stderr)
	}
}

// TestScaffoldNonInteractiveRejectsWhitespaceDescription asserts whitespace-only
// --description is rejected in non-interactive mode.
func TestScaffoldNonInteractiveRejectsWhitespaceDescription(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "Demo",
		"--description", "   ",
	)
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(stderr, "--description") {
		t.Errorf("stderr should mention --description: %q", stderr)
	}
}

func assertExecutable(t *testing.T, path string) {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if st.Mode().Perm()&0o111 == 0 {
		t.Errorf("%s is not executable (mode=%o)", path, st.Mode().Perm())
	}
}
