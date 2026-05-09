//nolint:testpackage // golden tests exercise unexported helpers (decodeRaw, asMap, ...) intentionally.
package configcli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// fixtureRoot points at tests/fixtures from the package directory. Tests are
// executed with CWD set to the package, so we walk three levels up.
const fixtureRoot = "../../../tests/fixtures"

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// runCmd invokes the cobra command and returns stdout, stderr, and the err sentinel.
func runCmd(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := NewCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestGoldenListSidecars(t *testing.T) {
	t.Parallel()
	got, stderr, err := runCmd(t, "list-sidecars", filepath.Join(fixtureRoot, "ci/pinned.workspace.toml"))
	if err != nil {
		t.Fatalf("Run err=%v stderr=%s", err, stderr)
	}
	want := mustRead(t, filepath.Join(fixtureRoot, "expected/list-sidecars/ci-pinned.txt"))
	if got != want {
		t.Errorf("list-sidecars output mismatch\nGOT:\n%s\nWANT:\n%s", got, want)
	}
}

func TestGoldenRepositoriesJSON(t *testing.T) {
	t.Parallel()
	got, stderr, err := runCmd(t, "repositories", filepath.Join(fixtureRoot, "ci/pinned.workspace.toml"))
	if err != nil {
		t.Fatalf("Run err=%v stderr=%s", err, stderr)
	}
	want := mustRead(t, filepath.Join(fixtureRoot, "expected/repositories/ci-pinned.txt"))
	if got != want {
		t.Errorf("repositories output mismatch\nGOT:\n%q\nWANT:\n%q", got, want)
	}
}

func TestGoldenDumpRepositories(t *testing.T) {
	t.Parallel()
	got, stderr, err := runCmd(t, "dump-repositories", filepath.Join(fixtureRoot, "ci/pinned.workspace.toml"))
	if err != nil {
		t.Fatalf("Run err=%v stderr=%s", err, stderr)
	}
	want := mustRead(t, filepath.Join(fixtureRoot, "expected/dump-repositories/ci-pinned.txt"))
	if got != want {
		t.Errorf("dump-repositories mismatch\nGOT:\n%s\nWANT:\n%s", got, want)
	}
}

func TestHasSection(t *testing.T) {
	t.Parallel()
	cases := []struct {
		section string
		want    string
	}{
		{"container", "true\n"},
		{"plugins", "true\n"},
		{"nonexistent", "false\n"},
	}
	for _, tc := range cases {
		t.Run(tc.section, func(t *testing.T) {
			t.Parallel()
			got, _, err := runCmd(t, "has-section",
				filepath.Join(fixtureRoot, "ci/pinned.workspace.toml"), tc.section)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestValidateWorkspace(t *testing.T) {
	t.Parallel()
	got, stderr, err := runCmd(t, "validate-workspace",
		filepath.Join(fixtureRoot, "ci/pinned.workspace.toml"))
	if err != nil {
		t.Fatalf("Run err=%v stderr=%s", err, stderr)
	}
	if want := "OK: " + filepath.Join(fixtureRoot, "ci/pinned.workspace.toml") + " is valid\n"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
