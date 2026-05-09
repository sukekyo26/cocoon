package envfile_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/envfile"
)

// minimalCtx returns the smallest WorkspaceContext that envfile.Generate
// will accept. ProjectDir is left to the caller because it is the field
// under test.
func minimalCtx(projectDir string) *generate.WorkspaceContext {
	return &generate.WorkspaceContext{
		ProjectDir: projectDir,
		WS: &config.Workspace{
			Container: config.ContainerSpec{
				ServiceName: "dev",
				Username:    "developer",
				Os:          "ubuntu",
				OsVersion:   "24.04",
			},
		},
	}
}

func TestGenerate_ProjectNameFromProjectDirBasename(t *testing.T) {
	t.Parallel()

	body, err := envfile.Generate(minimalCtx("/home/me/MyProject"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// Directory basename is lowercased so docker-compose accepts it
	// (project names must be lowercase).
	if !strings.Contains(body, "COMPOSE_PROJECT_NAME=myproject\n") {
		t.Errorf("expected COMPOSE_PROJECT_NAME=myproject, got:\n%s", body)
	}
	// CONTAINER_SERVICE_NAME stays bound to service_name.
	if !strings.Contains(body, "CONTAINER_SERVICE_NAME=dev\n") {
		t.Errorf("expected CONTAINER_SERVICE_NAME=dev, got:\n%s", body)
	}
}

func TestGenerate_ProjectNameFallsBackToServiceName(t *testing.T) {
	t.Parallel()

	// Empty ProjectDir is treated as "caller did not migrate yet"; we
	// fall back to ServiceName so generated `.env` always has both
	// fields populated.
	body, err := envfile.Generate(minimalCtx(""))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(body, "COMPOSE_PROJECT_NAME=dev\n") {
		t.Errorf("expected fallback COMPOSE_PROJECT_NAME=dev, got:\n%s", body)
	}
}

func TestGenerate_ProjectNameDotFallsBackToServiceName(t *testing.T) {
	t.Parallel()

	// "." would produce filepath.Base("") == "." which is not a valid
	// compose project name; fall back rather than emitting a literal dot.
	body, err := envfile.Generate(minimalCtx("."))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(body, "COMPOSE_PROJECT_NAME=dev\n") {
		t.Errorf("expected fallback COMPOSE_PROJECT_NAME=dev, got:\n%s", body)
	}
}

func TestGenerate_ProjectNameAlreadyLowercaseBasename(t *testing.T) {
	t.Parallel()

	body, err := envfile.Generate(minimalCtx("/home/me/cocoon"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(body, "COMPOSE_PROJECT_NAME=cocoon\n") {
		t.Errorf("expected COMPOSE_PROJECT_NAME=cocoon, got:\n%s", body)
	}
}
