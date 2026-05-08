package devcontainercompose_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/devcontainercompose"
)

func TestGenerateGoldenSnapshot(t *testing.T) {
	t.Parallel()
	ctx := &generate.WorkspaceContext{
		WS: &config.Workspace{
			Container: config.ContainerSpec{ServiceName: "snapshot-test"},
		},
	}
	got, err := devcontainercompose.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "snapshot.expected"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}

func TestGenerateDefaultServiceName(t *testing.T) {
	t.Parallel()
	ctx := &generate.WorkspaceContext{WS: &config.Workspace{}}
	got, err := devcontainercompose.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(got, "  dev:\n") {
		t.Errorf("expected default service name 'dev', got:\n%s", got)
	}
}

func TestGenerateWithSidecars(t *testing.T) {
	t.Parallel()
	ctx := &generate.WorkspaceContext{
		WS: &config.Workspace{
			Container: config.ContainerSpec{ServiceName: "dev"},
			Services: map[string]config.SidecarService{
				"redis":    {Image: "redis:7"},
				"postgres": {Image: "postgres:16"},
			},
		},
	}
	got, err := devcontainercompose.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	want := "    depends_on:\n      - postgres\n      - redis\n"
	if !strings.Contains(got, want) {
		t.Errorf("depends_on block not found:\n%s", got)
	}
}

func TestSafeNameQuoting(t *testing.T) {
	t.Parallel()
	ctx := &generate.WorkspaceContext{
		WS: &config.Workspace{
			Container: config.ContainerSpec{ServiceName: "with space"},
		},
	}
	got, err := devcontainercompose.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(got, `  "with space":`) {
		t.Errorf("expected quoted service name, got:\n%s", got)
	}
}
