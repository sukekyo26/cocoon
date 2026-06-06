package plugincli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
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

func TestScaffoldGoldenInstaller(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stdout, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--template", "installer",
		"--version-capable",
		"--name", "Demo",
		"--description", "Demo plugin",
		"--url", "https://example.com",
		"--non-interactive",
	)
	if err != nil {
		t.Fatalf("Run err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "OK:") {
		t.Errorf("stdout missing OK marker: %q", stdout)
	}
	assertGoldenDir(t, filepath.Join(dir, "demo"),
		"testdata/scaffold-installer",
		[]string{"plugin.toml", "install.installer.sh"})
	assertExecutable(t, filepath.Join(dir, "demo", "install.installer.sh"))
}

func TestScaffoldGoldenBinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--template", "binary",
		"--version-capable",
		"--requires-root",
		"--with-install-user",
		"--name", "Demo Bin",
		"--description", "Demo binary plugin",
		"--url", "https://example.com/bin",
		"--non-interactive",
	)
	if err != nil {
		t.Fatalf("Run err=%v stderr=%s", err, stderr)
	}
	assertGoldenDir(t, filepath.Join(dir, "demo"),
		"testdata/scaffold-binary",
		[]string{"plugin.toml", "install.binary.sh", "install_user.sh"})
	assertExecutable(t, filepath.Join(dir, "demo", "install.binary.sh"))
	assertExecutable(t, filepath.Join(dir, "demo", "install_user.sh"))
}

func TestScaffoldRejectsInvalidID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, err := runCmd(t,
		"scaffold", "Bad_ID",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "x",
		"--description", "x",
		"--url", "https://x.example.com",
	)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "Invalid plugin id") {
		t.Errorf("err does not mention id rule: %v", err)
	}
	// Errors must flow through the returned value, not be written to stderr by
	// scaffold itself — the binary boundary (main.go) prints them once.
	if stderr != "" {
		t.Errorf("scaffold must not write errors to stderr directly: %q", stderr)
	}
}

// Positive auto-discovery: when cwd is inside a cocoon project,
// `cocoon plugin scaffold <id>` without --plugins-dir lands the new plugin at
// <workspace>/.cocoon/plugins/<id>/. Locks the BREAKING default change so a
// future refactor that re-introduces ./plugins/<id>/ is caught.
//
//nolint:paralleltest // t.Chdir mutates process-wide state; cannot run in parallel.
func TestScaffoldAutoDiscoversWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "workspace.toml"), []byte("[plugins]\nenable = []\n"), 0o600); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	t.Chdir(root)

	_, stderr, err := runCmd(t,
		"scaffold", "demo",
		"--non-interactive",
		"--name", "Demo",
		"--description", "Demo",
		"--url", "https://example.com",
		"--template", "apt",
	)
	if err != nil {
		t.Fatalf("scaffold: err=%v stderr=%s", err, stderr)
	}
	wantDir := filepath.Join(root, ".cocoon", "plugins", "demo")
	if _, statErr := os.Stat(filepath.Join(wantDir, "plugin.toml")); statErr != nil {
		t.Errorf("expected plugin.toml at %s: %v", wantDir, statErr)
	}
	if _, statErr := os.Stat(filepath.Join(wantDir, "install.apt.sh")); statErr != nil {
		t.Errorf("expected install.apt.sh at %s: %v", wantDir, statErr)
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
	_, _, err := runCmd(t,
		"scaffold", "demo",
		"--non-interactive",
		"--name", "Demo",
		"--description", "Demo",
		"--url", "https://example.com",
	)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "--plugins-dir") {
		t.Errorf("err should mention --plugins-dir: %v", err)
	}
}

func TestScaffoldRequiresIDArg(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, _, err := runCmd(t,
		"scaffold",
		"--plugins-dir", dir,
		"--non-interactive",
	)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if err.Error() == "" {
		t.Errorf("expected non-empty error message")
	}
}

func TestScaffoldRequiresForceOnExistingDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "demo"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, _, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "Demo",
		"--description", "Demo",
		"--url", "https://example.com",
	)
	if !errors.Is(err, clihelpers.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("err should mention --force: %v", err)
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
		"--description", "Demo",
		"--url", "https://example.com",
	)
	if err != nil {
		t.Fatalf("Run err=%v stderr=%s", err, stderr)
	}
	body := mustRead(t, stale)
	if strings.Contains(body, "# stale") {
		t.Errorf("plugin.toml was not overwritten: %q", body)
	}
}

func TestScaffoldRejectsBinaryWithoutVersionCapable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, _, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--template", "binary",
		"--non-interactive",
		"--name", "Demo",
		"--description", "Demo",
		"--url", "https://example.com",
	)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "binary") {
		t.Errorf("err should mention binary: %v", err)
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
		"--description", "Demo",
		"--url", "https://example.com",
	)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestScaffoldRequiresName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, _, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--description", "Demo",
		"--url", "https://example.com",
	)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("err should mention name flag: %v", err)
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
	if !errors.Is(err, clihelpers.ErrUsage) {
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
	_, _, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "Demo",
		"--description", "Demo",
		"--url", "https://example.com",
		"--template", "apt",
	)
	if !errors.Is(err, clihelpers.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
	if !strings.Contains(err.Error(), "demo") {
		t.Errorf("expected err to mention path: %v", err)
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
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

// TestScaffoldNonInteractiveRejectsWhitespaceName asserts that a whitespace-only
// --name is rejected with ErrUsage in non-interactive mode (i.e. validateNameInput
// is applied symmetrically with the interactive form).
func TestScaffoldNonInteractiveRejectsWhitespaceName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, _, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "   ",
		"--description", "Demo",
		"--url", "https://example.com",
	)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "--name") {
		t.Errorf("err should mention --name: %v", err)
	}
}

// TestScaffoldNonInteractiveRejectsMissingURL asserts that omitting --url in
// non-interactive mode is rejected with ErrUsage that names --url.
func TestScaffoldNonInteractiveRejectsMissingURL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, _, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "Demo",
		"--description", "Demo",
		// no --url
	)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "--url") {
		t.Errorf("err should mention --url: %v", err)
	}
}

// TestScaffoldNonInteractiveRejectsMalformedURL asserts that a non-https or
// whitespace-containing --url is rejected in non-interactive mode.
func TestScaffoldNonInteractiveRejectsMalformedURL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, _, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "Demo",
		"--description", "Demo",
		"--url", "http://insecure.example.com",
	)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "--url") {
		t.Errorf("err should mention --url: %v", err)
	}
}

// TestScaffoldNonInteractiveRejectsWhitespaceDescription asserts whitespace-only
// --description is rejected in non-interactive mode.
func TestScaffoldNonInteractiveRejectsWhitespaceDescription(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, _, err := runCmd(t,
		"scaffold", "demo",
		"--plugins-dir", dir,
		"--non-interactive",
		"--name", "Demo",
		"--description", "   ",
		"--url", "https://example.com",
	)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "--description") {
		t.Errorf("err should mention --description: %v", err)
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
