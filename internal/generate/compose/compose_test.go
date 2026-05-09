package compose_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/compose"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

func TestGenerate_Snapshot(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	wsPath := filepath.Join(repoRoot, "tests", "fixtures", "snapshot.workspace.toml")
	pluginsDir := filepath.Join(repoRoot, "internal", "plugin", "catalog")

	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}

	var warns bytes.Buffer
	plugins, err := plugin.LoadEnabled(pluginsDir, ws.Plugins.Enable, &warns)
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}
	if cerr := plugin.CheckConflicts(plugins); cerr != nil {
		t.Fatalf("plugin conflicts: %v", cerr)
	}

	ctx := &generate.WorkspaceContext{WS: ws, PluginsDir: pluginsDir, Plugins: plugins, Warnings: &warns}
	got, err := compose.Generate(ctx, compose.Options{Plugins: plugins, Warnings: &warns})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	wantBytes, err := os.ReadFile(filepath.Join("testdata", "snapshot.expected"))
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if got != string(wantBytes) {
		t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, string(wantBytes))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// internal/generate/compose -> repo root
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}
