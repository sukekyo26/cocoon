package codeworkspace_test

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/codeworkspace"
)

// updateGolden, when set with `go test -update-golden`, rewrites the
// testdata/*.expected files from the current generator output. Mirrors the
// devcontainerjson / dockerfile package convention.
//
//nolint:gochecknoglobals // test-only flag scoped to codeworkspace_test.
var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/*.expected from current generator output")

// fixedDirs returns a deterministic projectDir / homeDir pair so golden
// file relative-path arithmetic stays stable across machines. Both live
// under a common parent so "~/.foo" relativized against projectDir gets a
// "../home/.foo" form that exercises the upward-traversal contract.
func fixedDirs() (projectDir, home string) {
	return "/tmp/cw-fixture/project", "/tmp/cw-fixture/home"
}

func newCtx(spec *config.CodeWorkspaceSpec) *generate.WorkspaceContext {
	projectDir, _ := fixedDirs()
	return &generate.WorkspaceContext{
		WS:         &config.Workspace{CodeWorkspace: spec},
		ProjectDir: projectDir,
	}
}

func runGenerate(t *testing.T, spec *config.CodeWorkspaceSpec, opts codeworkspace.Options) string {
	t.Helper()
	_, home := fixedDirs()
	if opts.HomeDir == "" {
		opts.HomeDir = home
	}
	out, err := codeworkspace.Generate(newCtx(spec), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return out
}

func compareGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if werr := os.WriteFile(path, []byte(got), 0o600); werr != nil {
			t.Fatalf("update golden %s: %v", name, werr)
		}
		return
	}
	want, err := os.ReadFile(path) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	if got != string(want) {
		t.Errorf("golden %s mismatch (run with -update-golden to refresh)\n--- got ---\n%s\n--- want ---\n%s", name, got, string(want))
	}
}

// basicSpec is the all-features fixture: 4 folders (project + home subdir +
// sibling + home subdir with explicit name override) + settings +
// extensions. Used by TestGenerateGoldenBasic.
func basicSpec() *config.CodeWorkspaceSpec {
	return &config.CodeWorkspaceSpec{
		Name: "test-stack",
		Folders: []config.CodeWorkspaceFolder{
			{Path: "."},
			{Path: "~/.claude"},
			{Path: "../sibling-repo"},
			{Path: "~/.config/nvim", Name: "Neovim"},
		},
		Settings: map[string]any{
			"editor.tabSize":  int64(2),
			"files.autoSave":  "afterDelay",
			"editor.codeLens": true,
		},
		Extensions: &config.CodeWorkspaceExtSpec{
			Recommendations: []string{"golang.go", "ms-azuretools.vscode-docker"},
		},
	}
}

func TestGenerateGoldenBasic(t *testing.T) {
	t.Parallel()
	got := runGenerate(t, basicSpec(), codeworkspace.Options{})
	compareGolden(t, "basic.expected", got)
}

// TestGenerateGoldenFoldersOnly covers the "settings / extensions elided"
// claim: when both inputs are empty the corresponding JSON keys are absent
// from the output, not emitted as "{}" or "[]".
func TestGenerateGoldenFoldersOnly(t *testing.T) {
	t.Parallel()
	spec := &config.CodeWorkspaceSpec{
		Folders: []config.CodeWorkspaceFolder{{Path: "."}},
	}
	got := runGenerate(t, spec, codeworkspace.Options{})
	compareGolden(t, "folders_only.expected", got)
}

// TestGenerateGoldenHomeTraversal pins the "../home/<rest>" shape produced
// when a folder uses "~" against the synthetic ProjectDir/HomeDir pair.
// This is the contract the user cares about: "~/.claude" must come out as
// a relative path VS Code can resolve, not a literal "~" or an absolute
// "/home/...".
func TestGenerateGoldenHomeTraversal(t *testing.T) {
	t.Parallel()
	spec := &config.CodeWorkspaceSpec{
		Folders: []config.CodeWorkspaceFolder{{Path: "~/.claude"}},
	}
	got := runGenerate(t, spec, codeworkspace.Options{})
	compareGolden(t, "home_traversal.expected", got)
}

// TestGenerateExtraFoldersAppendedAfterTomlList asserts that opts.ExtraFolders
// (the CLI --folder flag bridge) are appended after the workspace.toml
// folders so the user's declarative order leads.
func TestGenerateExtraFoldersAppendedAfterTomlList(t *testing.T) {
	t.Parallel()
	spec := &config.CodeWorkspaceSpec{
		Folders: []config.CodeWorkspaceFolder{{Path: "."}},
	}
	got := runGenerate(t, spec, codeworkspace.Options{
		ExtraFolders: []config.CodeWorkspaceFolder{
			{Path: "../sibling-repo"},
		},
	})
	idxFirst := strings.Index(got, `"name": "project"`)
	idxSecond := strings.Index(got, `"name": "sibling-repo"`)
	if idxFirst < 0 || idxSecond < 0 || idxFirst >= idxSecond {
		t.Errorf("expected toml entry before extra entry; got: %s", got)
	}
}

// TestGenerateTrailingNewline pins the docstring claim "ends with a single
// trailing newline".
func TestGenerateTrailingNewline(t *testing.T) {
	t.Parallel()
	got := runGenerate(t, basicSpec(), codeworkspace.Options{})
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("output must end with newline")
	}
	if strings.HasSuffix(got, "\n\n") {
		t.Fatalf("output must end with exactly one newline, not two")
	}
}

// TestGenerateTwoSpaceIndent pins the docstring claim "2-space indent".
func TestGenerateTwoSpaceIndent(t *testing.T) {
	t.Parallel()
	got := runGenerate(t, basicSpec(), codeworkspace.Options{})
	// Pick a line that must be indented (the first folder entry's name key).
	// json.Indent with "  " produces leading "  " or "    " etc. — never tabs.
	if !strings.Contains(got, "  \"folders\"") {
		t.Errorf("expected 2-space-indented \"folders\" key; got: %s", got)
	}
	if strings.Contains(got, "\t") {
		t.Errorf("output must not contain tab characters; got: %s", got)
	}
}

// TestGenerateProducesValidJSON pins the implicit contract: the output is
// valid JSON consumable by VS Code's parser. A broken indent / encoder
// step would slip past golden if shapes line up by accident.
func TestGenerateProducesValidJSON(t *testing.T) {
	t.Parallel()
	got := runGenerate(t, basicSpec(), codeworkspace.Options{})
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, got)
	}
	if _, ok := parsed["folders"]; !ok {
		t.Errorf("parsed JSON missing \"folders\" key")
	}
}

// TestGenerateErrors exercises every sentinel exported by the package. Each
// case must classify via errors.Is — string-matching the message would tie
// tests to the wording instead of the contract.
func TestGenerateErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		spec *config.CodeWorkspaceSpec
		opts codeworkspace.Options
		want error
	}{
		{
			name: "no folders in spec, no extras",
			spec: &config.CodeWorkspaceSpec{},
			opts: codeworkspace.Options{},
			want: codeworkspace.ErrNoFolders,
		},
		{
			name: "nil spec, no extras",
			spec: nil,
			opts: codeworkspace.Options{},
			want: codeworkspace.ErrNoFolders,
		},
		{
			name: "empty folders[].path",
			spec: &config.CodeWorkspaceSpec{
				Folders: []config.CodeWorkspaceFolder{{Path: ""}},
			},
			opts: codeworkspace.Options{},
			want: codeworkspace.ErrInvalidFolderPath,
		},
		{
			name: "~user form rejected",
			spec: &config.CodeWorkspaceSpec{
				Folders: []config.CodeWorkspaceFolder{{Path: "~bob/.config"}},
			},
			opts: codeworkspace.Options{},
			want: codeworkspace.ErrInvalidFolderPath,
		},
		{
			name: "~ expansion without HomeDir",
			spec: &config.CodeWorkspaceSpec{
				Folders: []config.CodeWorkspaceFolder{{Path: "~/.claude"}},
			},
			opts: codeworkspace.Options{HomeDir: ""},
			want: codeworkspace.ErrMissingHomeDir,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := newCtx(tc.spec)
			// Only inject the default home for cases that need it; the
			// "no home" case must keep opts.HomeDir empty.
			opts := tc.opts
			if opts.HomeDir == "" && !errors.Is(tc.want, codeworkspace.ErrMissingHomeDir) {
				_, home := fixedDirs()
				opts.HomeDir = home
			}
			_, err := codeworkspace.Generate(ctx, opts)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want errors.Is %v", err, tc.want)
			}
		})
	}
}

// TestGeneratePathVariations exercises the resolver's input categories.
// Each row asserts the path comes out at a specific JSON-encoded value;
// the resolver is the only place where the projectDir-relative arithmetic
// lives, so this is the failure-surface to pin.
func TestGeneratePathVariations(t *testing.T) {
	t.Parallel()
	projectDir, _ := fixedDirs()
	parentBase := filepath.Base(filepath.Dir(projectDir))
	cases := []struct {
		name   string
		folder config.CodeWorkspaceFolder
		// JSON-encoded path value (no surrounding quotes); matched as a substring.
		wantPath string
		wantName string
	}{
		{
			name:     "project root (.)",
			folder:   config.CodeWorkspaceFolder{Path: "."},
			wantPath: `"path": "."`,
			wantName: `"name": "project"`,
		},
		{
			name:     "absolute path",
			folder:   config.CodeWorkspaceFolder{Path: "/etc/nginx"},
			wantPath: `"path": "../../../etc/nginx"`,
			wantName: `"name": "nginx"`,
		},
		{
			name:     "relative parent traversal",
			folder:   config.CodeWorkspaceFolder{Path: "../sibling"},
			wantPath: `"path": "../sibling"`,
			wantName: `"name": "sibling"`,
		},
		{
			name:     "home expansion (~/foo)",
			folder:   config.CodeWorkspaceFolder{Path: "~/.bashrc"},
			wantPath: `"path": "../home/.bashrc"`,
			wantName: `"name": ".bashrc"`,
		},
		{
			name:     "home root (~)",
			folder:   config.CodeWorkspaceFolder{Path: "~"},
			wantPath: `"path": "../home"`,
			wantName: `"name": "home"`,
		},
		{
			name:     "explicit name override",
			folder:   config.CodeWorkspaceFolder{Path: "../sibling", Name: "Sibling"},
			wantPath: `"path": "../sibling"`,
			wantName: `"name": "Sibling"`,
		},
	}
	// Touch parentBase to silence "declared and not used" if the test grows
	// to inspect it later. Today the cases above bake the expected strings
	// directly because they read better at the call-site than a derived
	// template.
	_ = parentBase
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spec := &config.CodeWorkspaceSpec{
				Folders: []config.CodeWorkspaceFolder{tc.folder},
			}
			got := runGenerate(t, spec, codeworkspace.Options{})
			if !strings.Contains(got, tc.wantPath) {
				t.Errorf("missing %s\n--- got ---\n%s", tc.wantPath, got)
			}
			if !strings.Contains(got, tc.wantName) {
				t.Errorf("missing %s\n--- got ---\n%s", tc.wantName, got)
			}
		})
	}
}

// TestGenerateSettingsEmptyKeyAbsent pins the docstring claim that an empty
// Settings map elides the "settings" key entirely (vs. emitting "{}").
func TestGenerateSettingsEmptyKeyAbsent(t *testing.T) {
	t.Parallel()
	spec := &config.CodeWorkspaceSpec{
		Folders:  []config.CodeWorkspaceFolder{{Path: "."}},
		Settings: map[string]any{},
	}
	got := runGenerate(t, spec, codeworkspace.Options{})
	if strings.Contains(got, "settings") {
		t.Errorf("output must not contain \"settings\" when Settings map is empty; got: %s", got)
	}
}

// TestGenerateExtensionsEmptyKeyAbsent is the parallel for extensions:
// when Recommendations is empty the entire "extensions" object is elided,
// not emitted as `"extensions": {}`.
func TestGenerateExtensionsEmptyKeyAbsent(t *testing.T) {
	t.Parallel()
	spec := &config.CodeWorkspaceSpec{
		Folders:    []config.CodeWorkspaceFolder{{Path: "."}},
		Extensions: &config.CodeWorkspaceExtSpec{Recommendations: nil},
	}
	got := runGenerate(t, spec, codeworkspace.Options{})
	if strings.Contains(got, "extensions") {
		t.Errorf("output must not contain \"extensions\" when Recommendations is empty; got: %s", got)
	}
}
