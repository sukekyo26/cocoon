package plugin_test

import (
	"bytes"
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

func TestLayeredFS_MaterializePicksWinningLayer(t *testing.T) {
	t.Parallel()

	embedded := makeEmbedded()
	userDir := writeUserOverlay(t)
	projectDir := writeProjectOverlay(t)
	layered := plugin.NewLayeredFS(embedded, userDir, projectDir)

	dst := t.TempDir()
	if err := plugin.Materialize(layered, []string{"alpha", "bravo"}, dst); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	// alpha must come from project (only plugin.toml exists there).
	alphaToml, err := os.ReadFile(filepath.Join(dst, "alpha", "plugin.toml"))
	if err != nil {
		t.Fatalf("read alpha plugin.toml: %v", err)
	}
	if !strings.Contains(string(alphaToml), "from project") {
		t.Errorf("alpha plugin.toml not from project: %q", string(alphaToml))
	}
	// bravo install.sh must come from user (and be 0o755).
	bravoSh := filepath.Join(dst, "bravo", "install.sh")
	body, err := os.ReadFile(bravoSh) //nolint:gosec
	if err != nil {
		t.Fatalf("read bravo install.sh: %v", err)
	}
	if !strings.Contains(string(body), "bravo-user") {
		t.Errorf("bravo install.sh not from user: %q", string(body))
	}
	info, err := os.Stat(bravoSh)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o755 {
		t.Errorf("install.sh perm: got %o, want 0755", mode)
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
