package plugin_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

// makeEmbedded fakes the embedded catalog with two plugins.
func makeEmbedded() fs.FS {
	return fstest.MapFS{
		"alpha/plugin.toml": &fstest.MapFile{Data: []byte("# alpha embedded\n")},
		"alpha/install.sh":  &fstest.MapFile{Data: []byte("echo alpha-embedded\n"), Mode: 0o755},
		"bravo/plugin.toml": &fstest.MapFile{Data: []byte("# bravo embedded\n")},
		"bravo/install.sh":  &fstest.MapFile{Data: []byte("echo bravo-embedded\n"), Mode: 0o755},
	}
}

// writeUserOverlay writes a user override for `bravo` and a brand-new `delta`.
func writeUserOverlay(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, p := range []struct {
		rel  string
		body string
	}{
		{"bravo/plugin.toml", "# bravo from user\n"},
		{"bravo/install.sh", "echo bravo-user\n"},
		{"delta/plugin.toml", "# delta from user\n"},
		{"delta/install.sh", "echo delta-user\n"},
	} {
		full := filepath.Join(dir, p.rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(p.body), 0o644); err != nil { //nolint:gosec
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

// writeProjectOverlay writes a project override for `alpha`.
func writeProjectOverlay(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	full := filepath.Join(dir, "alpha", "plugin.toml")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte("# alpha from project\n"), 0o644); err != nil { //nolint:gosec
		t.Fatalf("write: %v", err)
	}
	return dir
}

func TestNewLayeredFS_PriorityOrder(t *testing.T) {
	t.Parallel()

	embedded := makeEmbedded()
	userDir := writeUserOverlay(t)
	projectDir := writeProjectOverlay(t)

	layered := plugin.NewLayeredFS(embedded, userDir, projectDir)

	want := map[string]string{
		"alpha": plugin.SourceProject, // project beats user (absent here) and embedded
		"bravo": plugin.SourceUser,    // user beats embedded
		"delta": plugin.SourceUser,    // user-only plugin
	}
	for id, src := range want {
		if got := layered.Source(id); got != src {
			t.Errorf("Source(%q) = %q, want %q", id, got, src)
		}
	}
}

func TestNewLayeredFS_OnlyEmbeddedWhenOverlaysAbsent(t *testing.T) {
	t.Parallel()

	layered := plugin.NewLayeredFS(makeEmbedded(), "", "")
	for _, id := range []string{"alpha", "bravo"} {
		if got := layered.Source(id); got != plugin.SourceEmbedded {
			t.Errorf("Source(%q) = %q, want embedded", id, got)
		}
	}
}

func TestLayeredFS_OpenAndReadDirRouteToWinningLayer(t *testing.T) {
	t.Parallel()

	embedded := makeEmbedded()
	userDir := writeUserOverlay(t)
	layered := plugin.NewLayeredFS(embedded, userDir, "")

	// bravo/install.sh must come from the user overlay, not the embedded one.
	body, err := fs.ReadFile(layered, "bravo/install.sh")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(body), "bravo-user") {
		t.Errorf("user overlay not honored: got %q", string(body))
	}

	// alpha/install.sh stays embedded since neither overlay covers it.
	body, err = fs.ReadFile(layered, "alpha/install.sh")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(body), "alpha-embedded") {
		t.Errorf("embedded fallback failed: got %q", string(body))
	}

	// ReadDir(".") returns all visible top-level ids.
	ents, err := fs.ReadDir(layered, ".")
	if err != nil {
		t.Fatalf("ReadDir(.): %v", err)
	}
	got := make([]string, 0, len(ents))
	for _, e := range ents {
		got = append(got, e.Name())
	}
	sort.Strings(got)
	wantIDs := []string{"alpha", "bravo", "delta"}
	if strings.Join(got, ",") != strings.Join(wantIDs, ",") {
		t.Errorf("root ids: got %v, want %v", got, wantIDs)
	}
}

// methodOverlayTOML is reused by the 3-layer method-resolution tests
// below. It declares two methods so cross-layer scenarios can fail on a
// missing script even if one of them happens to exist somewhere.
const methodOverlayTOML = `[metadata]
name = "multi"
description = "two-method fixture"
url = "https://example.test/multi"
default = false

[install]
requires_root = false
default_method = "official"

[install.methods.official]
description = "Official installer"

[install.methods.binary]
description = "Direct binary"

[version]
version_capable = false
`

// TestLayeredFS_MethodScriptsResolveInWinningLayer pins the architectural
// guarantee the design plan calls "同 layer 完結": LayeredFS routes every
// file under a plugin id (plugin.toml AND install.<method>.sh) through the
// one layer that owns that id, so a multi-method plugin must ship its
// declarations and all method scripts together in the same layer.
//
// The two subtests cover both sides of the contract:
//
//   - same_layer_visible: all files in user overlay → all readable through
//     LayeredFS at that id.
//   - cross_layer_hidden: plugin.toml in project (the winner) but
//     install.<method>.sh only in user → method scripts NOT readable, even
//     though they physically exist somewhere; layerFor(id) selected project
//     and never looks at user for the same id.
func TestLayeredFS_MethodScriptsResolveInWinningLayer(t *testing.T) {
	t.Parallel()

	t.Run("same_layer_visible", func(t *testing.T) {
		t.Parallel()
		userDir := t.TempDir()
		writePluginDir(t, userDir, "multi", map[string]string{
			"plugin.toml":         methodOverlayTOML,
			"install.official.sh": "#!/bin/sh\necho official-user\n",
			"install.binary.sh":   "#!/bin/sh\necho binary-user\n",
		})
		// Embedded ships only an unrelated plugin so the LayeredFS root
		// is non-empty but never wins for "multi".
		embedded := fstest.MapFS{
			"alpha/plugin.toml": &fstest.MapFile{Data: []byte("# alpha embedded\n")},
			"alpha/install.sh":  &fstest.MapFile{Data: []byte("echo alpha\n"), Mode: 0o755},
		}
		layered := plugin.NewLayeredFS(embedded, userDir, "")

		if got := layered.Source("multi"); got != plugin.SourceUser {
			t.Fatalf("Source(multi) = %q, want user", got)
		}
		for _, rel := range []string{
			"multi/plugin.toml",
			"multi/install.official.sh",
			"multi/install.binary.sh",
		} {
			body, err := fs.ReadFile(layered, rel)
			if err != nil {
				t.Errorf("ReadFile(%q) = %v, want success", rel, err)
				continue
			}
			if len(body) == 0 {
				t.Errorf("ReadFile(%q) returned empty body", rel)
			}
		}

		// LoadEnabledFromFS — the path cocoon gen actually uses — must
		// succeed end-to-end: validateMethodScripts walks the SAME
		// LayeredFS, so all method scripts are visible at the winning
		// layer and the loader accepts the plugin.
		plugins, err := plugin.LoadEnabledFromFS(layered, []string{"multi"}, nil, "")
		if err != nil {
			t.Fatalf("LoadEnabledFromFS: %v", err)
		}
		p, ok := plugins["multi"]
		if !ok {
			t.Fatalf("LoadEnabledFromFS returned no entry for multi: %v", plugins)
		}
		if len(p.Install.Methods) != 2 {
			t.Errorf("Methods = %v, want 2 declared", p.Install.Methods)
		}
	})

	t.Run("cross_layer_hidden", func(t *testing.T) {
		t.Parallel()
		// User overlay has the method scripts but NOT the plugin.toml.
		userDir := t.TempDir()
		writePluginDir(t, userDir, "multi", map[string]string{
			"install.official.sh": "#!/bin/sh\necho official-user\n",
			"install.binary.sh":   "#!/bin/sh\necho binary-user\n",
		})
		// Project overlay has plugin.toml only. layerFor("multi") will
		// pick project (highest priority) and never read from user, so
		// the method scripts effectively vanish at this id.
		projectDir := t.TempDir()
		writePluginDir(t, projectDir, "multi", map[string]string{
			"plugin.toml": methodOverlayTOML,
		})
		layered := plugin.NewLayeredFS(fstest.MapFS{}, userDir, projectDir)

		if got := layered.Source("multi"); got != plugin.SourceProject {
			t.Fatalf("Source(multi) = %q, want project", got)
		}
		// plugin.toml is reachable from project.
		if _, err := fs.ReadFile(layered, "multi/plugin.toml"); err != nil {
			t.Fatalf("ReadFile(plugin.toml): %v", err)
		}
		// Method scripts are invisible — even though user overlay has
		// them on disk, LayeredFS routes "multi/*" through project only.
		for _, rel := range []string{
			"multi/install.official.sh",
			"multi/install.binary.sh",
		} {
			if _, err := fs.ReadFile(layered, rel); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("ReadFile(%q) err = %v, want fs.ErrNotExist (cross-layer must NOT resolve)", rel, err)
			}
		}

		// End-to-end: LoadEnabledFromFS surfaces this as a method-script
		// validation failure, so cross-layer splits fail loud at load
		// time instead of silently shipping a half-installed plugin.
		// The validator aggregates multiple "does not exist" errors and
		// renders the trailing ones as "(and N more)", so the assertion
		// checks for the leading entry + the aggregation marker.
		_, err := plugin.LoadEnabledFromFS(layered, []string{"multi"}, nil, "")
		if err == nil {
			t.Fatal("LoadEnabledFromFS unexpectedly succeeded for cross-layer split")
		}
		msg := err.Error()
		for _, want := range []string{"does not exist", "and 1 more"} {
			if !strings.Contains(msg, want) {
				t.Errorf("error %q missing substring %q", msg, want)
			}
		}
		// At least one of the two method script names must appear by
		// name; the validator picks one deterministically as the
		// "leader" and stuffs the other into "and N more".
		if !strings.Contains(msg, "install.official.sh") && !strings.Contains(msg, "install.binary.sh") {
			t.Errorf("error %q must name at least one of install.official.sh / install.binary.sh", msg)
		}
	})
}

// writePluginDir materialises one plugin directory under root with the
// given <relative-file>:<body> entries. Empty maps create the dir but no
// files (used to seed an id whose plugin.toml lives in *another* layer).
func writePluginDir(t *testing.T, root, id string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	for rel, body := range files {
		full := filepath.Join(dir, rel)
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil { //nolint:gosec // testdata
			t.Fatalf("write %s: %v", full, err)
		}
	}
}

// makeThreePluginLayered builds a LayeredFS whose root has three top-level
// ids ("alpha", "bravo", "delta"), used by the synthetic-root tests below.
func makeThreePluginLayered(t *testing.T) *plugin.LayeredFS {
	t.Helper()
	embedded := makeEmbedded() // alpha, bravo
	userDir := writeUserOverlay(t)
	return plugin.NewLayeredFS(embedded, userDir, "") // adds delta
}

func TestLayeredFS_OpenRootReturnsSyntheticDir(t *testing.T) {
	t.Parallel()

	layered := makeThreePluginLayered(t)

	f, err := layered.Open(".")
	if err != nil {
		t.Fatalf("Open(.): %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("Stat().IsDir() = false, want true")
	}
	if info.Name() != "." {
		t.Errorf("Stat().Name() = %q, want %q", info.Name(), ".")
	}
	if info.Mode()&fs.ModeDir == 0 {
		t.Errorf("Stat().Mode() = %v, missing fs.ModeDir", info.Mode())
	}
	if info.Size() != 0 {
		t.Errorf("Stat().Size() = %d, want 0 for synthetic dir", info.Size())
	}
	if !info.ModTime().IsZero() {
		t.Errorf("Stat().ModTime() = %v, want zero", info.ModTime())
	}
	if info.Sys() != nil {
		t.Errorf("Stat().Sys() = %v, want nil", info.Sys())
	}

	// Open(".") must satisfy fs.ReadDirFile so callers like fs.WalkDir work.
	if _, ok := f.(fs.ReadDirFile); !ok {
		t.Errorf("Open(.) returned %T, does not implement fs.ReadDirFile", f)
	}
}

func TestLayeredFS_OpenRootReadIsDirectoryError(t *testing.T) {
	t.Parallel()

	layered := makeThreePluginLayered(t)
	f, err := layered.Open(".")
	if err != nil {
		t.Fatalf("Open(.): %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	buf := make([]byte, 16)
	_, readErr := f.Read(buf)
	if readErr == nil {
		t.Fatal("Read on synthetic root returned nil err, want is-a-directory PathError")
	}
	var pe *fs.PathError
	if !errors.As(readErr, &pe) {
		t.Fatalf("Read err = %T (%v), want *fs.PathError", readErr, readErr)
	}
	if pe.Op != "read" || pe.Path != "." {
		t.Errorf("PathError = {Op:%q Path:%q}, want {read .}", pe.Op, pe.Path)
	}
	if !strings.Contains(pe.Err.Error(), "is a directory") {
		t.Errorf("PathError.Err = %v, want substring %q", pe.Err, "is a directory")
	}
}

func TestLayeredFS_RootReadDirPagination(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		ns       []int
		wantLens []int
		wantEOF  bool
	}{
		{name: "one_at_a_time", ns: []int{1, 1, 1, 1}, wantLens: []int{1, 1, 1, 0}, wantEOF: true},
		{name: "two_then_remainder", ns: []int{2, 2}, wantLens: []int{2, 1}, wantEOF: false},
		{name: "n_zero_returns_all", ns: []int{0}, wantLens: []int{3}, wantEOF: false},
		{name: "n_negative_returns_all", ns: []int{-1}, wantLens: []int{3}, wantEOF: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			layered := makeThreePluginLayered(t)
			f, err := layered.Open(".")
			if err != nil {
				t.Fatalf("Open(.): %v", err)
			}
			t.Cleanup(func() { _ = f.Close() })
			rd, ok := f.(fs.ReadDirFile)
			if !ok {
				t.Fatalf("Open(.) returned %T, want fs.ReadDirFile", f)
			}

			var lastErr error
			for i, n := range tc.ns {
				ents, err := rd.ReadDir(n)
				lastErr = err
				if len(ents) != tc.wantLens[i] {
					t.Errorf("ReadDir(%d)[step %d] len = %d, want %d", n, i, len(ents), tc.wantLens[i])
				}
			}
			if tc.wantEOF {
				if !errors.Is(lastErr, io.EOF) {
					t.Errorf("final ReadDir err = %v, want io.EOF", lastErr)
				}
			} else if lastErr != nil {
				t.Errorf("final ReadDir err = %v, want nil", lastErr)
			}
		})
	}
}

func TestLayeredFS_InvalidPathRejected(t *testing.T) {
	t.Parallel()

	layered := makeThreePluginLayered(t)

	cases := []struct {
		name string
		op   string
		path string
	}{
		{name: "open_parent_escape", op: "open", path: "../x"},
		{name: "open_double_slash", op: "open", path: "foo//bar"},
		{name: "open_absolute", op: "open", path: "/abs"},
		{name: "readdir_parent_escape", op: "readdir", path: "../x"},
		{name: "readdir_double_slash", op: "readdir", path: "foo//bar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var err error
			switch tc.op {
			case "open":
				_, err = layered.Open(tc.path)
			case "readdir":
				_, err = layered.ReadDir(tc.path)
			}
			if err == nil {
				t.Fatalf("%s(%q) err = nil, want fs.ErrInvalid", tc.op, tc.path)
			}
			if !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("%s(%q) err = %v, want errors.Is fs.ErrInvalid", tc.op, tc.path, err)
			}
		})
	}
}

func TestLayeredFS_ReadDirUnknownPluginID(t *testing.T) {
	t.Parallel()

	layered := makeThreePluginLayered(t)
	_, err := layered.ReadDir("ghost")
	if err == nil {
		t.Fatal("ReadDir(ghost) err = nil, want fs.ErrNotExist")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadDir(ghost) err = %v, want errors.Is fs.ErrNotExist", err)
	}
}

func TestLayeredFS_RootFileClose(t *testing.T) {
	t.Parallel()

	layered := makeThreePluginLayered(t)
	f, err := layered.Open(".")
	if err != nil {
		t.Fatalf("Open(.): %v", err)
	}
	if err := f.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

func TestLogOverrides_OnlyOverriddenIDs(t *testing.T) {
	t.Parallel()

	embedded := makeEmbedded()
	userDir := writeUserOverlay(t)
	projectDir := writeProjectOverlay(t)
	layered := plugin.NewLayeredFS(embedded, userDir, projectDir)

	var buf bytes.Buffer
	layered.LogOverrides(&buf)

	want := "INFO: plugin alpha overridden by project\n" +
		"INFO: plugin bravo overridden by user\n" +
		"INFO: plugin delta overridden by user\n"
	if got := buf.String(); got != want {
		t.Errorf("LogOverrides:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
