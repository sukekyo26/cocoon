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

func TestUpsertPinLine(t *testing.T) {
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
			if upErr := plugin.UpsertPinLine(tmp, tc.id, tc.ref, tc.amd64Sum, tc.arm64Sum); upErr != nil {
				t.Fatalf("UpsertPinLine: %v", upErr)
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

func TestUpsertPinLineRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertPinLine(path, "", "1.0", "", ""); !errors.Is(err, plugin.ErrPinLineEmptyID) {
		t.Errorf("empty id: got %v, want ErrPinLineEmptyID", err)
	}
	if err := plugin.UpsertPinLine(path, "go", "", "", ""); !errors.Is(err, plugin.ErrPinLineEmptyRef) {
		t.Errorf("empty ref: got %v, want ErrPinLineEmptyRef", err)
	}
}

// Whitespace-only "blank" lines (e.g. "  " or "\t") between an existing
// inline-table line and the next section must round-trip verbatim through
// the replace path. The renderLines re-emit must preserve the exact
// blank-line whitespace, not normalize to empty strings.
func TestUpsertPinLineReplacePreservesWhitespaceBlankLines(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	// "  " (two-space) and "\t" (tab) lines between go's pin and [mounts].
	body := []byte("[plugins.versions]\ngo = { pin = \"1.22.0\" }\n  \n\t\n[mounts]\nhost = \"./src\"\n")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertPinLine(path, "go", "1.23.4", "", ""); err != nil {
		t.Fatalf("UpsertPinLine: %v", err)
	}
	got, err := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "[plugins.versions]\ngo = { pin = \"1.23.4\" }\n  \n\t\n[mounts]\nhost = \"./src\"\n"
	if string(got) != want {
		t.Errorf("whitespace-only blank lines were not preserved verbatim\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// Trailing blank lines outside the modified section must be preserved
// verbatim. The append-new-section path must not strip them.
func TestUpsertPinLinePreservesTrailingBlankLines(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	body := []byte("[plugins]\nenable = []\n\n\n") // two trailing blank lines
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertPinLine(path, "go", "1.23.4", "", ""); err != nil {
		t.Fatalf("UpsertPinLine: %v", err)
	}
	got, err := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "[plugins]\nenable = []\n\n\n[plugins.versions]\ngo = { pin = \"1.23.4\" }\n"
	if string(got) != want {
		t.Errorf("trailing blank lines were not preserved\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// A workspace.toml that came in without a trailing newline must come back
// without one. Regression guard for the renderLines newline convention.
func TestUpsertPinLinePreservesNoTrailingNewline(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	body := []byte(`[plugins]
enable = ["go"]`) // no trailing \n
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertPinLine(path, "go", "1.23.4", "", ""); err != nil {
		t.Fatalf("UpsertPinLine: %v", err)
	}
	got, err := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) == 0 || got[len(got)-1] == '\n' {
		t.Errorf("trailing newline was added (source had none):\n%q", string(got))
	}
}

// UpsertPinLine must not silently relax workspace.toml's permissions.
func TestUpsertPinLinePreservesFileMode(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	if err := os.WriteFile(path, []byte("[plugins]\nenable = []\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertPinLine(path, "go", "1.23.4", "", ""); err != nil {
		t.Fatalf("UpsertPinLine: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want 0600", perm)
	}
}

func TestUpsertMethodLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		seed, want string
		id, method string
	}{
		{
			// Empty input has no trailing newline; renderLines preserves
			// that convention (the output ends right after the value).
			name:   "empty-file",
			seed:   "",
			id:     "copilot-cli",
			method: "binary",
			want:   "[plugins.methods]\ncopilot-cli = \"binary\"",
		},
		{
			name:   "no-methods-section-appends-with-separator",
			seed:   "[plugins]\nenable = [\"copilot-cli\"]\n",
			id:     "copilot-cli",
			method: "binary",
			want:   "[plugins]\nenable = [\"copilot-cli\"]\n\n[plugins.methods]\ncopilot-cli = \"binary\"\n",
		},
		{
			name:   "add-alongside-existing-id",
			seed:   "[plugins.methods]\nfoo = \"a\"\n",
			id:     "bar",
			method: "b",
			want:   "[plugins.methods]\nfoo = \"a\"\nbar = \"b\"\n",
		},
		{
			name:   "replace-existing-id-preserves-other-lines",
			seed:   "[plugins.methods]\ncopilot-cli = \"gh-cli\"\nfoo = \"a\"\n",
			id:     "copilot-cli",
			method: "binary",
			want:   "[plugins.methods]\ncopilot-cli = \"binary\"\nfoo = \"a\"\n",
		},
		{
			name:   "preserves-trailing-blank-line-when-section-absent",
			seed:   "[plugins]\nenable = []\n\n\n",
			id:     "copilot-cli",
			method: "binary",
			// Existing trailing blanks are preserved verbatim, so separation > 1.
			want: "[plugins]\nenable = []\n\n\n[plugins.methods]\ncopilot-cli = \"binary\"\n",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmp := t.TempDir()
			path := filepath.Join(tmp, "ws.toml")
			if err := os.WriteFile(path, []byte(tc.seed), 0o600); err != nil {
				t.Fatalf("seed: %v", err)
			}
			if err := plugin.UpsertMethodLine(path, tc.id, tc.method); err != nil {
				t.Fatalf("UpsertMethodLine: %v", err)
			}
			got, err := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("mismatch\n--- got ---\n%q\n--- want ---\n%q", got, tc.want)
			}
		})
	}
}

func TestUpsertMethodLineRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertMethodLine(path, "", "binary"); !errors.Is(err, plugin.ErrMethodLineEmptyID) {
		t.Errorf("empty id: got %v, want ErrMethodLineEmptyID", err)
	}
	if err := plugin.UpsertMethodLine(path, "copilot-cli", ""); !errors.Is(err, plugin.ErrMethodLineEmptyMethod) {
		t.Errorf("empty method: got %v, want ErrMethodLineEmptyMethod", err)
	}
}

// Preserve the source's trailing-newline convention through the
// methods mutator (regression guard mirroring the pin variant).
func TestUpsertMethodLinePreservesNoTrailingNewline(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	body := []byte("[plugins]\nenable = [\"copilot-cli\"]") // no trailing \n
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertMethodLine(path, "copilot-cli", "binary"); err != nil {
		t.Fatalf("UpsertMethodLine: %v", err)
	}
	got, err := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) == 0 || got[len(got)-1] == '\n' {
		t.Errorf("trailing newline was added (source had none):\n%q", string(got))
	}
}

// File mode must not silently relax through the methods mutator.
func TestUpsertMethodLinePreservesFileMode(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	if err := os.WriteFile(path, []byte("[plugins]\nenable = []\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertMethodLine(path, "copilot-cli", "binary"); err != nil {
		t.Fatalf("UpsertMethodLine: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want 0600", perm)
	}
}

// FormatMethodLine is the smallest unit of the methods writer; pin its
// shape so a refactor of the mutator does not silently change the
// stdout snippet shown by `cocoon plugin pin --method`.
func TestFormatMethodLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id, method, want string
	}{
		{"copilot-cli", "binary", "copilot-cli = \"binary\"\n"},
		{"a", "b", "a = \"b\"\n"},
		// Method name with a hyphen must round-trip through %q without escaping.
		{"foo", "gh-cli", "foo = \"gh-cli\"\n"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.id+"="+tc.method, func(t *testing.T) {
			t.Parallel()
			if got := plugin.FormatMethodLine(tc.id, tc.method); got != tc.want {
				t.Errorf("FormatMethodLine(%q, %q) = %q, want %q", tc.id, tc.method, got, tc.want)
			}
		})
	}
}

// TestUpsertPinAndMethod pins the combined mutator's promises:
//
//   - method == "" reduces to "pin only", matching UpsertPinLine output
//     byte-for-byte so the new function is a safe drop-in for callers that
//     never need to touch [plugins.methods].
//   - method != "" produces a single read-write cycle that lands both
//     sections together; the disk state never holds a pin without its
//     matching method (the regression this combined function exists to
//     prevent).
//   - empty id / ref → existing sentinel errors (ErrPinLineEmptyID /
//     ErrPinLineEmptyRef). Empty method is the "pin only" path, not an
//     error, so it must NOT trigger ErrMethodLineEmptyMethod.
func TestUpsertPinAndMethod(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		seed, want         string
		id, ref, method    string
		amd64Sum, arm64Sum string
		wantPinOnly        bool // when true, also assert UpsertPinLine alone produces the same bytes
	}{
		{
			name: "pin_and_method_on_empty_file",
			seed: "",
			id:   "copilot-cli", ref: "1.0.47", method: "binary",
			want: "[plugins.versions]\ncopilot-cli = { pin = \"1.0.47\" }\n\n[plugins.methods]\ncopilot-cli = \"binary\"",
		},
		{
			name: "pin_only_when_method_empty",
			seed: "",
			id:   "go", ref: "1.23.4", method: "",
			want:        "[plugins.versions]\ngo = { pin = \"1.23.4\" }",
			wantPinOnly: true,
		},
		{
			name: "replace_both_existing",
			seed: "[plugins.versions]\ncopilot-cli = { pin = \"1.0.46\" }\n\n[plugins.methods]\ncopilot-cli = \"gh-cli\"\n",
			id:   "copilot-cli", ref: "1.0.47", method: "binary",
			want: "[plugins.versions]\ncopilot-cli = { pin = \"1.0.47\" }\n\n[plugins.methods]\ncopilot-cli = \"binary\"\n",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmp := t.TempDir()
			path := filepath.Join(tmp, "ws.toml")
			if err := os.WriteFile(path, []byte(tc.seed), 0o600); err != nil {
				t.Fatalf("seed: %v", err)
			}
			if err := plugin.UpsertPinAndMethod(path, tc.id, tc.ref, tc.amd64Sum, tc.arm64Sum, tc.method); err != nil {
				t.Fatalf("UpsertPinAndMethod: %v", err)
			}
			got, err := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("mismatch\n--- got ---\n%q\n--- want ---\n%q", got, tc.want)
			}
			// Drop-in equivalence: when method == "" the combined writer must
			// produce the exact same bytes as the standalone UpsertPinLine.
			if tc.wantPinOnly {
				path2 := filepath.Join(tmp, "ws-pinonly.toml")
				if err := os.WriteFile(path2, []byte(tc.seed), 0o600); err != nil {
					t.Fatalf("seed pinonly: %v", err)
				}
				if err := plugin.UpsertPinLine(path2, tc.id, tc.ref, tc.amd64Sum, tc.arm64Sum); err != nil {
					t.Fatalf("UpsertPinLine: %v", err)
				}
				got2, gerr := os.ReadFile(path2) //nolint:gosec // tmp path under t.TempDir
				if gerr != nil {
					t.Fatalf("read pinonly: %v", gerr)
				}
				if string(got2) != string(got) {
					t.Errorf("pin-only path diverged from UpsertPinLine:\n--- combined ---\n%q\n--- standalone ---\n%q", got, got2)
				}
			}
		})
	}
}

func TestUpsertPinAndMethodRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertPinAndMethod(path, "", "1.0", "", "", "binary"); !errors.Is(err, plugin.ErrPinLineEmptyID) {
		t.Errorf("empty id: got %v, want ErrPinLineEmptyID", err)
	}
	if err := plugin.UpsertPinAndMethod(path, "go", "", "", "", "binary"); !errors.Is(err, plugin.ErrPinLineEmptyRef) {
		t.Errorf("empty ref: got %v, want ErrPinLineEmptyRef", err)
	}
	// Empty method is the "pin only" path; must succeed, not error.
	if err := plugin.UpsertPinAndMethod(path, "go", "1.0", "", "", ""); err != nil {
		t.Errorf("empty method should be the pin-only path, got error: %v", err)
	}
}

// TestUpsertPinAndMethodRejectsLegacySubsection guards the legacy-shape
// migration prompt across the combined writer too: a workspace.toml with
// `[plugins.versions.<id>]` must be rejected before any in-place edit so
// the user is not surprised by a partial-shape file.
func TestUpsertPinAndMethodRejectsLegacySubsection(t *testing.T) {
	t.Parallel()
	body := `[plugins]
enable = ["copilot-cli"]

[plugins.versions.copilot-cli]
pin = "1.0.46"
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	err := plugin.UpsertPinAndMethod(path, "copilot-cli", "1.0.47", "", "", "binary")
	if !errors.Is(err, plugin.ErrLegacyPinSubsection) {
		t.Errorf("got %v, want ErrLegacyPinSubsection", err)
	}
	got, rerr := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
	if rerr != nil {
		t.Fatalf("read after refusal: %v", rerr)
	}
	if string(got) != body {
		t.Errorf("workspace.toml was modified despite refusal:\n--- got ---\n%s", got)
	}
}

// Legacy `[plugins.versions.<id>]` subsection blocks (cocoon's previous
// emission form) collide with the new inline-table layout. The mutator
// must refuse so the user explicitly migrates the file.
func TestUpsertPinLineRejectsLegacySubsection(t *testing.T) {
	t.Parallel()
	body := `[plugins]
enable = ["go"]

[plugins.versions.go]
pin = "1.22.5"
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	err := plugin.UpsertPinLine(path, "go", "1.23.4", "", "")
	if !errors.Is(err, plugin.ErrLegacyPinSubsection) {
		t.Errorf("got %v, want ErrLegacyPinSubsection", err)
	}
	got, rerr := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
	if rerr != nil {
		t.Fatalf("read after refusal: %v", rerr)
	}
	if string(got) != body {
		t.Errorf("workspace.toml was modified despite refusal:\n--- got ---\n%s", got)
	}
}
