package fsx_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/fsx"
)

func TestEnsureGitignoreEntry(t *testing.T) {
	t.Parallel()

	const pattern = ".env.local"
	const comment = "# cocoon: secret"

	cases := []struct {
		name      string
		seed      *string // nil = file absent
		wantBody  string
		wantWrite bool
	}{
		{
			name:      "creates_when_absent",
			seed:      nil,
			wantBody:  "# cocoon: secret\n.env.local\n",
			wantWrite: true,
		},
		{
			name:      "appends_preserving_existing",
			seed:      ptr("# user rules\n*.log\nbuild/\n"),
			wantBody:  "# user rules\n*.log\nbuild/\n# cocoon: secret\n.env.local\n",
			wantWrite: true,
		},
		{
			name:      "adds_trailing_newline_before_append",
			seed:      ptr("*.log"), // no trailing newline
			wantBody:  "*.log\n# cocoon: secret\n.env.local\n",
			wantWrite: true,
		},
		{
			name:      "noop_when_already_present",
			seed:      ptr("*.log\n.env.local\nbuild/\n"),
			wantBody:  "*.log\n.env.local\nbuild/\n", // unchanged, verbatim
			wantWrite: false,
		},
		{
			name:      "noop_matches_indented_line",
			seed:      ptr("  .env.local  \n"), // trimmed match
			wantBody:  "  .env.local  \n",
			wantWrite: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), ".gitignore")
			if tc.seed != nil {
				if err := os.WriteFile(path, []byte(*tc.seed), 0o644); err != nil { //nolint:gosec // test fixture
					t.Fatalf("seed: %v", err)
				}
			}
			changed, err := fsx.EnsureGitignoreEntry(path, pattern, comment)
			if err != nil {
				t.Fatalf("EnsureGitignoreEntry: %v", err)
			}
			if changed != tc.wantWrite {
				t.Errorf("changed = %v, want %v", changed, tc.wantWrite)
			}
			got, err := os.ReadFile(path) //nolint:gosec // test path under t.TempDir
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if string(got) != tc.wantBody {
				t.Errorf("body =\n%q\nwant\n%q", got, tc.wantBody)
			}
		})
	}
}

// TestEnsureGitignoreEntry_Idempotent pins that a second call after a create
// makes no further change (no duplicate line).
func TestEnsureGitignoreEntry_Idempotent(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".gitignore")
	if _, err := fsx.EnsureGitignoreEntry(path, ".env.local", "# c"); err != nil {
		t.Fatalf("first: %v", err)
	}
	first, err := os.ReadFile(path) //nolint:gosec // test path under t.TempDir
	if err != nil {
		t.Fatalf("read first: %v", err)
	}
	changed, err := fsx.EnsureGitignoreEntry(path, ".env.local", "# c")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if changed {
		t.Error("second call reported a change; want no-op")
	}
	second, err := os.ReadFile(path) //nolint:gosec // test path under t.TempDir
	if err != nil {
		t.Fatalf("read second: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("second call modified the file:\nfirst=%q\nsecond=%q", first, second)
	}
}

// TestEnsureGitignoreEntry_Perm pins the permission contract: an upsert into an
// existing file preserves that file's mode (does not force 0644), while a
// freshly created file is 0644.
func TestEnsureGitignoreEntry_Perm(t *testing.T) {
	t.Parallel()

	t.Run("preserves_existing_mode", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), ".gitignore")
		// 0600 differs from the create default and is umask-robust.
		if err := os.WriteFile(path, []byte("*.log\n"), 0o600); err != nil { //nolint:gosec // test fixture
			t.Fatalf("seed: %v", err)
		}
		changed, err := fsx.EnsureGitignoreEntry(path, ".env.local", "# c")
		if err != nil {
			t.Fatalf("EnsureGitignoreEntry: %v", err)
		}
		if !changed {
			t.Fatal("changed = false, want true")
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("mode = %o, want 600 (existing perm not preserved)", got)
		}
	})

	t.Run("new_file_is_0644", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), ".gitignore")
		if _, err := fsx.EnsureGitignoreEntry(path, ".env.local", "# c"); err != nil {
			t.Fatalf("EnsureGitignoreEntry: %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if got := info.Mode().Perm(); got != 0o644 {
			t.Errorf("mode = %o, want 644", got)
		}
	})
}

// TestEnsureGitignoreEntry_EmptyComment pins the docstring claim that passing
// comment == "" skips the comment line — no stray blank line or lone newline
// before the pattern, on both create and append.
func TestEnsureGitignoreEntry_EmptyComment(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), ".gitignore")
		changed, err := fsx.EnsureGitignoreEntry(path, ".env.local", "")
		if err != nil {
			t.Fatalf("EnsureGitignoreEntry: %v", err)
		}
		if !changed {
			t.Fatal("changed = false, want true")
		}
		got, err := os.ReadFile(path) //nolint:gosec // test path under t.TempDir
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(got) != ".env.local\n" {
			t.Errorf("body = %q, want %q", got, ".env.local\n")
		}
	})

	t.Run("append", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), ".gitignore")
		if err := os.WriteFile(path, []byte("*.log\n"), 0o644); err != nil { //nolint:gosec // test fixture
			t.Fatalf("seed: %v", err)
		}
		if _, err := fsx.EnsureGitignoreEntry(path, ".env.local", ""); err != nil {
			t.Fatalf("EnsureGitignoreEntry: %v", err)
		}
		got, err := os.ReadFile(path) //nolint:gosec // test path under t.TempDir
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(got) != "*.log\n.env.local\n" {
			t.Errorf("body = %q, want %q", got, "*.log\n.env.local\n")
		}
	})
}

// TestEnsureGitignoreEntry_ReadError pins that a read failure surfaces as a
// non-nil error with changed=false — never a silent (false, nil) that would
// leave the secret-ignore line unwritten. A directory path stats OK but fails
// to read ("is a directory"), exercising that branch portably.
func TestEnsureGitignoreEntry_ReadError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir() // exists; os.ReadFile on a directory errors
	changed, err := fsx.EnsureGitignoreEntry(dir, ".env.local", "# c")
	if err == nil {
		t.Fatal("expected an error when path is a directory, got nil")
	}
	if changed {
		t.Error("changed = true on read error, want false")
	}
}

func ptr(s string) *string { return &s }
