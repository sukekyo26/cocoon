package plugin_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

// upsertPin upserts an enable entry alone through the combined writer
// (method == ""), the same path production uses; the standalone pin writer
// was removed.
func upsertPin(path, id, spec string) error {
	return plugin.UpsertPinAndMethod(path, id, spec, "")
}

// TestUpsertPin pins the enable-array upsert across the shapes that matter:
// creating [plugins] from nothing, replacing a bare id, replacing a pinned
// id, appending a new id, and leaving surrounding sections untouched. The
// array is always re-emitted in cocoon's canonical multi-line style.
func TestUpsertPin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		seed, want string
		id, spec   string
	}{
		{
			name: "empty_file_creates_section",
			seed: "",
			id:   "go", spec: "=1.23.4",
			want: "[plugins]\nenable = [\n    \"go=1.23.4\",\n]",
		},
		{
			name: "replace_bare_id",
			seed: "[plugins]\nenable = [\"go\"]\n",
			id:   "go", spec: "=1.23.4",
			want: "[plugins]\nenable = [\n    \"go=1.23.4\",\n]\n",
		},
		{
			name: "replace_pinned_id",
			seed: "[plugins]\nenable = [\n    \"go=1.22.0\",\n    \"node=22.0.0\",\n]\n",
			id:   "go", spec: "=1.24.0",
			want: "[plugins]\nenable = [\n    \"go=1.24.0\",\n    \"node=22.0.0\",\n]\n",
		},
		{
			name: "append_new_id",
			seed: "[plugins]\nenable = [\n    \"node=22.0.0\",\n]\n",
			id:   "go", spec: "latest",
			want: "[plugins]\nenable = [\n    \"node=22.0.0\",\n    \"go=latest\",\n]\n",
		},
		{
			name: "preserves_surrounding_sections",
			seed: "# top comment\n[plugins]\nenable = [\n    \"go=1.22.0\",\n]\n\n[plugins.methods]\ncopilot-cli = \"binary\"\n",
			id:   "go", spec: "=1.24.0",
			want: "# top comment\n[plugins]\nenable = [\n    \"go=1.24.0\",\n]\n\n[plugins.methods]\ncopilot-cli = \"binary\"\n",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "ws.toml")
			if err := os.WriteFile(path, []byte(tc.seed), 0o600); err != nil {
				t.Fatalf("seed: %v", err)
			}
			if err := upsertPin(path, tc.id, tc.spec); err != nil {
				t.Fatalf("upsertPin: %v", err)
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

func TestUpsertPinRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := upsertPin(path, "", "=1.0"); !errors.Is(err, plugin.ErrPinLineEmptyID) {
		t.Errorf("empty id: got %v, want ErrPinLineEmptyID", err)
	}
	if err := upsertPin(path, "go", ""); !errors.Is(err, plugin.ErrPinLineEmptyRef) {
		t.Errorf("empty spec: got %v, want ErrPinLineEmptyRef", err)
	}
}

// Whitespace-only "blank" lines (e.g. "  " or "\t") after the enable array
// must round-trip verbatim through the replace path. The renderLines re-emit
// must preserve the exact blank-line whitespace, not normalize it to empty.
func TestUpsertPinReplacePreservesWhitespaceBlankLines(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ws.toml")
	body := []byte("[plugins]\nenable = [\n    \"go=1.22.0\",\n]\n  \n\t\n[mounts]\nhost = \"./src\"\n")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := upsertPin(path, "go", "=1.23.4"); err != nil {
		t.Fatalf("upsertPin: %v", err)
	}
	got, err := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "[plugins]\nenable = [\n    \"go=1.23.4\",\n]\n  \n\t\n[mounts]\nhost = \"./src\"\n"
	if string(got) != want {
		t.Errorf("whitespace-only blank lines were not preserved verbatim\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// Trailing blank lines outside the modified section must be preserved
// verbatim through the in-place array replacement.
func TestUpsertPinPreservesTrailingBlankLines(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ws.toml")
	body := []byte("[plugins]\nenable = []\n\n\n") // two trailing blank lines
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := upsertPin(path, "go", "=1.23.4"); err != nil {
		t.Fatalf("upsertPin: %v", err)
	}
	got, err := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "[plugins]\nenable = [\n    \"go=1.23.4\",\n]\n\n\n"
	if string(got) != want {
		t.Errorf("trailing blank lines were not preserved\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// A workspace.toml that came in without a trailing newline must come back
// without one. Regression guard for the renderLines newline convention.
func TestUpsertPinPreservesNoTrailingNewline(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ws.toml")
	body := []byte(`[plugins]
enable = ["go"]`) // no trailing \n
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := upsertPin(path, "go", "=1.23.4"); err != nil {
		t.Fatalf("upsertPin: %v", err)
	}
	got, err := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) == 0 || got[len(got)-1] == '\n' {
		t.Errorf("trailing newline was added (source had none):\n%q", string(got))
	}
}

// upsertPin must not silently relax workspace.toml's permissions.
func TestUpsertPinPreservesFileMode(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ws.toml")
	if err := os.WriteFile(path, []byte("[plugins]\nenable = []\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := upsertPin(path, "go", "=1.23.4"); err != nil {
		t.Fatalf("upsertPin: %v", err)
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
//   - method == "" reduces to "enable only" — only the [plugins].enable entry
//     is written and [plugins.methods] is left untouched.
//   - method != "" produces a single read-write cycle that lands both the
//     enable entry and the [plugins.methods] line together; the disk state
//     never holds a pin without its matching method.
//   - empty id / spec → existing sentinel errors. Empty method is the
//     "enable only" path, not an error.
func TestUpsertPinAndMethod(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		seed, want       string
		id, spec, method string
	}{
		{
			name: "pin_and_method_on_empty_file",
			seed: "",
			id:   "copilot-cli", spec: "=1.0.47", method: "binary",
			want: "[plugins]\nenable = [\n    \"copilot-cli=1.0.47\",\n]\n\n[plugins.methods]\ncopilot-cli = \"binary\"",
		},
		{
			name: "pin_only_when_method_empty",
			seed: "",
			id:   "go", spec: "=1.23.4", method: "",
			want: "[plugins]\nenable = [\n    \"go=1.23.4\",\n]",
		},
		{
			name: "replace_both_existing",
			seed: "[plugins]\nenable = [\n    \"copilot-cli=1.0.46\",\n]\n\n[plugins.methods]\ncopilot-cli = \"gh-cli\"\n",
			id:   "copilot-cli", spec: "=1.0.47", method: "binary",
			want: "[plugins]\nenable = [\n    \"copilot-cli=1.0.47\",\n]\n\n[plugins.methods]\ncopilot-cli = \"binary\"\n",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "ws.toml")
			if err := os.WriteFile(path, []byte(tc.seed), 0o600); err != nil {
				t.Fatalf("seed: %v", err)
			}
			if err := plugin.UpsertPinAndMethod(path, tc.id, tc.spec, tc.method); err != nil {
				t.Fatalf("UpsertPinAndMethod: %v", err)
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

func TestUpsertPinAndMethodRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := plugin.UpsertPinAndMethod(path, "", "=1.0", "binary"); !errors.Is(err, plugin.ErrPinLineEmptyID) {
		t.Errorf("empty id: got %v, want ErrPinLineEmptyID", err)
	}
	if err := plugin.UpsertPinAndMethod(path, "go", "", "binary"); !errors.Is(err, plugin.ErrPinLineEmptyRef) {
		t.Errorf("empty spec: got %v, want ErrPinLineEmptyRef", err)
	}
	// Empty method is the "enable only" path; must succeed, not error.
	if err := plugin.UpsertPinAndMethod(path, "go", "=1.0", ""); err != nil {
		t.Errorf("empty method should be the enable-only path, got error: %v", err)
	}
}

// TestUpsertPinRejectsLegacyVersions guards the migration prompt: a
// workspace.toml that still carries a [plugins.versions] section (or the older
// [plugins.versions.<id>] subsection) must be rejected before any in-place
// edit so the user is not left with an inconsistent file.
func TestUpsertPinRejectsLegacyVersions(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, body string }{
		{"section", "[plugins]\nenable = [\"go\"]\n\n[plugins.versions]\ngo = \"=1.22.5\"\n"},
		{"subsection", "[plugins]\nenable = [\"go\"]\n\n[plugins.versions.go]\npin = \"1.22.5\"\n"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "ws.toml")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatalf("seed: %v", err)
			}
			err := plugin.UpsertPinAndMethod(path, "go", "=1.23.4", "")
			if !errors.Is(err, plugin.ErrLegacyPluginVersions) {
				t.Errorf("got %v, want ErrLegacyPluginVersions", err)
			}
			got, rerr := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
			if rerr != nil {
				t.Fatalf("read after refusal: %v", rerr)
			}
			if string(got) != tc.body {
				t.Errorf("workspace.toml was modified despite refusal:\n--- got ---\n%s", got)
			}
		})
	}
}

// TestUpsertPinPreservesOptions pins the separation of concerns: bumping a
// plugin's version touches only its enable entry. A [plugins.options] table
// carrying that plugin's [install.extra_versions] knobs is left verbatim.
func TestUpsertPinPreservesOptions(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ws.toml")
	body := "[plugins]\nenable = [\n    \"android-sdk=14742923\",\n]\n\n" +
		"[plugins.options]\nandroid-sdk = { api_level = \"35\", build_tools = \"35.0.0\" }\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := upsertPin(path, "android-sdk", "=14999999"); err != nil {
		t.Fatalf("upsertPin: %v", err)
	}
	got, rerr := os.ReadFile(path) //nolint:gosec // tmp path under t.TempDir
	if rerr != nil {
		t.Fatalf("read: %v", rerr)
	}
	want := "[plugins]\nenable = [\n    \"android-sdk=14999999\",\n]\n\n" +
		"[plugins.options]\nandroid-sdk = { api_level = \"35\", build_tools = \"35.0.0\" }\n"
	if string(got) != want {
		t.Errorf("options were not preserved:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
