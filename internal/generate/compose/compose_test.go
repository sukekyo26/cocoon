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
	pluginsDir := filepath.Join(repoRoot, "internal", "plugin", "catalog")

	// Two fixtures lock both mount_root branches: ".." emits the
	// sibling-repo bind mount (`../..:/home/.../workspace`) and "."
	// emits the project-only mount plus the working_dir override.
	cases := []struct {
		name        string
		fixture     string
		expectation string
	}{
		{
			name:        "parent_mount",
			fixture:     "snapshot.workspace.toml",
			expectation: "snapshot.expected",
		},
		{
			name:        "cwd_mount",
			fixture:     "snapshot-cwd.workspace.toml",
			expectation: "snapshot-cwd.expected",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wsPath := filepath.Join(repoRoot, "tests", "fixtures", tc.fixture)
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

			ctx := &generate.WorkspaceContext{
				WS: ws, PluginsFS: os.DirFS(pluginsDir), Plugins: plugins, Warnings: &warns,
			}
			got, err := compose.Generate(ctx, compose.Options{Plugins: plugins, Warnings: &warns})
			if err != nil {
				t.Fatalf("generate: %v", err)
			}

			wantBytes, err := os.ReadFile(filepath.Join("testdata", tc.expectation))
			if err != nil {
				t.Fatalf("read expected: %v", err)
			}
			if got != string(wantBytes) {
				t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, string(wantBytes))
			}
		})
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
