package devcontainerjson_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/devcontainerjson"
)

func TestGenerateGoldenSnapshot(t *testing.T) {
	t.Parallel()
	ws, err := config.LoadWorkspace(filepath.Join("..", "..", "..", "tests", "fixtures", "snapshot.workspace.toml"))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	ctx := &generate.WorkspaceContext{WS: ws}
	got, err := devcontainerjson.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "snapshot.expected"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}

func TestGenerateMinimalDefaults(t *testing.T) {
	t.Parallel()
	ctx := &generate.WorkspaceContext{WS: &config.Workspace{}}
	got, err := devcontainerjson.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, want := range []string{
		`"service": "dev"`,
		`"workspaceFolder": "/home/developer/workspace"`,
		`"forwardPorts": [`,
		`"shutdownAction": "stopCompose"`,
		`"extensions": []`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output:\n%s", want, got)
		}
	}
}

func TestGenerateForwardPortsUnion(t *testing.T) {
	t.Parallel()
	ctx := &generate.WorkspaceContext{
		WS: &config.Workspace{
			Ports: &config.PortsSpec{Forward: []any{"3000:3000", "5173:5173"}},
			Devcontainer: config.Devcontainer{
				"forwardPorts": []any{int64(5173), int64(9000)},
			},
		},
	}
	got, err := devcontainerjson.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	want := "\"forwardPorts\": [\n\t\t3000,\n\t\t5173,\n\t\t9000\n\t]"
	if !strings.Contains(got, want) {
		t.Errorf("expected union %q, got:\n%s", want, got)
	}
}

// TestGenerateForwardPortsSkipsNonInt verifies that compose-only entries
// (port ranges, host-mode long-form) are skipped from forwardPorts and a
// warning is emitted to ctx.Warnings, but the remaining single-port entries
// still flow through.
func TestGenerateForwardPortsSkipsNonInt(t *testing.T) {
	t.Parallel()
	var warn strings.Builder
	ctx := &generate.WorkspaceContext{
		Warnings: &warn,
		WS: &config.Workspace{
			Ports: &config.PortsSpec{Forward: []any{
				"3000:3000",
				"3000-3005:3000-3005",
				map[string]any{"target": int64(8080), "mode": "host"},
				map[string]any{"target": int64(5432), "published": int64(15432)},
			}},
		},
	}
	got, err := devcontainerjson.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(got, "\"forwardPorts\": [\n\t\t3000,\n\t\t15432\n\t]") {
		t.Errorf("forwardPorts should be [3000, 15432], got:\n%s", got)
	}
	if !strings.Contains(warn.String(), "uses a port range") {
		t.Errorf("expected range warning, got %q", warn.String())
	}
	if !strings.Contains(warn.String(), `uses mode = "host"`) {
		t.Errorf("expected host-mode warning, got %q", warn.String())
	}
}

func TestGenerateDeepMergeReplacesScalarsAndLists(t *testing.T) {
	t.Parallel()
	ctx := &generate.WorkspaceContext{
		WS: &config.Workspace{
			Devcontainer: config.Devcontainer{
				"name": "Custom Name",
				"customizations": map[string]any{
					"vscode": map[string]any{
						"extensions": []any{"foo.bar"},
						"settings":   map[string]any{"editor.tabSize": int64(4)},
					},
				},
			},
		},
	}
	got, err := devcontainerjson.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, want := range []string{
		`"name": "Custom Name"`,
		`"extensions": [` + "\n\t\t\t\t\"foo.bar\"\n\t\t\t]",
		`"settings": {`,
		`"editor.tabSize": 4`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in:\n%s", want, got)
		}
	}
}
