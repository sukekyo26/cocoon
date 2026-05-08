package repositories_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/repositories"
)

func TestValidatePath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	cases := []struct {
		name string
		rel  string
		ok   bool
	}{
		{"plain dir", "foo", true},
		{"nested dir", "foo/bar", true},
		{"empty rejected", "", false},
		{"absolute rejected", "/etc/passwd", false},
		{"dot-dot rejected", "../escape", false},
		{"middle dot-dot rejected", "foo/../../escape", false},
		{"workspace-docker rejected", "workspace-docker", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := repositories.ValidatePath(tmp, tc.rel)
			if tc.ok && err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected error, got %q", got)
			}
		})
	}
}

func writeWorkspace(t *testing.T, body string) string {
	t.Helper()
	scriptDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(scriptDir, "workspace.toml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return scriptDir
}

const baseToml = `[container]
service_name = "dev"
username = "dev"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = []

`

func TestStatus_MissingAndNotGitAndOK(t *testing.T) {
	t.Parallel()
	scriptDir := writeWorkspace(t, baseToml+`[repositories]
clone = [
  { url = "https://example.com/a.git", path = "a" },
  { url = "https://example.com/b.git", path = "b" },
  { url = "https://example.com/c.git", path = "c" },
]
`)
	parent := filepath.Dir(scriptDir)
	// "a" missing, "b" exists but no .git, "c" with .git dir
	if err := os.MkdirAll(filepath.Join(parent, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(parent, "c", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := repositories.CheckStatus(scriptDir)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d, want 3", len(got))
	}
	want := []repositories.Status{
		repositories.StatusMissing,
		repositories.StatusNotGit,
		repositories.StatusOK,
	}
	for i, r := range got {
		if r.Status != want[i] {
			t.Errorf("entry %d status=%s want=%s", i, r.Status, want[i])
		}
	}
}

func TestLoadEntries_NoSection(t *testing.T) {
	t.Parallel()
	scriptDir := writeWorkspace(t, baseToml)
	got, err := repositories.LoadEntries(scriptDir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != nil {
		t.Fatalf("want nil, got %#v", got)
	}
}

func TestLoadEntries_ResolvesPath(t *testing.T) {
	t.Parallel()
	scriptDir := writeWorkspace(t, baseToml+`[repositories]
clone = [
  { url = "https://github.com/x/y.git" },
  { url = "git@github.com:foo/bar.git", branch = "main", depth = 1, recurse_submodules = true },
]
`)
	got, err := repositories.LoadEntries(scriptDir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Path != "y" {
		t.Errorf("entry0 path=%q want=y", got[0].Path)
	}
	if got[1].Path != "bar" || got[1].Branch != "main" || got[1].Depth != 1 || !got[1].RecurseSubmodules {
		t.Errorf("entry1=%#v", got[1])
	}
}

func TestCloneAll_IdempotentSkip(t *testing.T) {
	t.Parallel()
	scriptDir := writeWorkspace(t, baseToml+`[repositories]
clone = [
  { url = "https://example.com/a.git", path = "a" },
]
`)
	parent := filepath.Dir(scriptDir)
	if err := os.MkdirAll(filepath.Join(parent, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	summary, err := repositories.CloneAll(exec.New(), scriptDir, nil)
	if err != nil {
		t.Fatalf("CloneAll: %v", err)
	}
	if summary.Skipped != 1 || summary.Cloned != 0 || summary.Failed != 0 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestStatus_NoFile(t *testing.T) {
	t.Parallel()
	got, err := repositories.CheckStatus(t.TempDir())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != nil {
		t.Fatalf("want nil, got %#v", got)
	}
}

func TestCloneAll_NoEntries(t *testing.T) {
	t.Parallel()
	scriptDir := writeWorkspace(t, baseToml)
	summary, err := repositories.CloneAll(exec.New(), scriptDir, nil)
	if err != nil {
		t.Fatalf("CloneAll: %v", err)
	}
	if summary.Cloned != 0 || summary.Skipped != 0 || summary.Failed != 0 {
		t.Errorf("summary should be zero, got %+v", summary)
	}
}

func TestCloneAll_NilLoggerOK(t *testing.T) {
	t.Parallel()
	scriptDir := writeWorkspace(t, baseToml+`[repositories]
clone = [
  { url = "https://example.com/a.git", path = "x" },
]
`)
	parent := filepath.Dir(scriptDir)
	// Pre-create the target so the clone is skipped without invoking git.
	if err := os.MkdirAll(filepath.Join(parent, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := repositories.CloneAll(exec.New(), scriptDir, nil); err != nil {
		t.Fatalf("CloneAll: %v", err)
	}
}
