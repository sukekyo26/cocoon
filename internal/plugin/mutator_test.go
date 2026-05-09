package plugin_test

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

var updateGolden = flag.Bool("update-golden", false, "update mutator golden files")

func TestUpsertPinBlock(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		id, ref  string
		amd64Sum string
		arm64Sum string
	}{
		{name: "empty-file", id: "go", ref: "1.23.4"},
		{
			name: "no-versions-section", id: "go", ref: "1.23.4",
			amd64Sum: "deadbeef", arm64Sum: "cafebabe",
		},
		{name: "add-alongside-existing-id", id: "uv", ref: "0.5.7"},
		{name: "replace-existing-id", id: "go", ref: "1.24.0"},
		{name: "preserve-comment-before-block", id: "go", ref: "1.23.4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			beforePath := filepath.Join("testdata", "mutator", tc.name, "before.toml")
			afterPath := filepath.Join("testdata", "mutator", tc.name, "after.toml")
			before, err := os.ReadFile(beforePath) //nolint:gosec // testdata
			if err != nil {
				t.Fatalf("read %s: %v", beforePath, err)
			}
			tmp := filepath.Join(t.TempDir(), "ws.toml")
			if writeErr := os.WriteFile(tmp, before, 0o600); writeErr != nil { //nolint:gosec // tmp path under t.TempDir
				t.Fatalf("seed: %v", writeErr)
			}
			if upErr := plugin.UpsertPinBlock(tmp, tc.id, tc.ref, tc.amd64Sum, tc.arm64Sum); upErr != nil {
				t.Fatalf("UpsertPinBlock: %v", upErr)
			}
			got, err := os.ReadFile(tmp) //nolint:gosec // tmp
			if err != nil {
				t.Fatalf("read result: %v", err)
			}
			if *updateGolden {
				if writeErr := os.WriteFile(afterPath, got, 0o644); writeErr != nil { //nolint:gosec // testdata
					t.Fatalf("write golden %s: %v", afterPath, writeErr)
				}
				return
			}
			want, err := os.ReadFile(afterPath) //nolint:gosec // testdata
			if err != nil {
				t.Fatalf("read %s: %v", afterPath, err)
			}
			if string(got) != string(want) {
				t.Errorf("golden mismatch (run with -update-golden to refresh)\n--- got ---\n%s\n--- want ---\n%s",
					got, want)
			}
		})
	}
}

func TestUpsertPinBlockRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertPinBlock(path, "", "1.0", "", ""); !errors.Is(err, plugin.ErrPinBlockEmptyID) {
		t.Errorf("empty id: got %v, want ErrPinBlockEmptyID", err)
	}
	if err := plugin.UpsertPinBlock(path, "go", "", "", ""); !errors.Is(err, plugin.ErrPinBlockEmptyRef) {
		t.Errorf("empty ref: got %v, want ErrPinBlockEmptyRef", err)
	}
}

// A workspace.toml that came in without a trailing newline must come back
// without one. Regression guard for the original renderLines bug where the
// "len(lines) > 0" branch unconditionally appended a final '\n'.
func TestUpsertPinBlockPreservesNoTrailingNewline(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	body := []byte(`[plugins]
enable = ["go"]`) // no trailing \n
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertPinBlock(path, "go", "1.23.4", "", ""); err != nil {
		t.Fatalf("UpsertPinBlock: %v", err)
	}
	got, err := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) == 0 || got[len(got)-1] == '\n' {
		t.Errorf("trailing newline was added (source had none):\n%q", string(got))
	}
}

// UpsertPinBlock must not silently relax workspace.toml's permissions. If the
// user kept their workspace.toml at 0600, --write should rewrite the file at
// 0600, not the previous hard-coded 0644.
func TestUpsertPinBlockPreservesFileMode(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	if err := os.WriteFile(path, []byte("[plugins]\nenable = []\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertPinBlock(path, "go", "1.23.4", "", ""); err != nil {
		t.Fatalf("UpsertPinBlock: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want 0600", perm)
	}
}

// When workspace.toml uses the inline-table form (`[plugins.versions]` +
// `<id> = { ... }`), the line-based mutator must refuse rather than append a
// duplicate `[plugins.versions.<id>]` block (which would produce a TOML
// duplicate-key error at gen time).
func TestUpsertPinBlockRejectsInlineForm(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	body := []byte(`[plugins]
enable = ["go"]

[plugins.versions]
go = { pin = "1.22.5" }
`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertPinBlock(path, "go", "1.23.4", "", ""); !errors.Is(err, plugin.ErrPinBlockInlineForm) {
		t.Errorf("inline form: got %v, want ErrPinBlockInlineForm", err)
	}
	got, err := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
	if err != nil {
		t.Fatalf("read after refusal: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("workspace.toml was modified despite refusal:\n--- got ---\n%s", got)
	}
}
