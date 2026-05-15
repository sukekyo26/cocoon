package envfile_test

import (
	"errors"
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
				ServiceName:  "dev",
				Username:     "developer",
				Image:        "ubuntu",
				ImageVersion: "24.04",
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

// TestGenerate_OmitsHostSpecificKeys pins the host-independent contract:
// the generated .env carries no UID/GID/DOCKER_GID, so committing it does
// not pin the generator's host identity onto the rest of the team.
func TestGenerate_OmitsHostSpecificKeys(t *testing.T) {
	t.Parallel()

	body, err := envfile.Generate(minimalCtx("/home/me/proj"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, line := range strings.Split(body, "\n") {
		key, _, _ := strings.Cut(line, "=")
		switch key {
		case "UID", "GID", "DOCKER_GID":
			t.Errorf("host-specific key %q must not appear in .env:\n%s", key, body)
		}
	}
}

// TestGenerate_KeepsWorkspaceKeys verifies the team-stable keys — all derived
// from workspace.toml or the project directory name — survive.
func TestGenerate_KeepsWorkspaceKeys(t *testing.T) {
	t.Parallel()

	body, err := envfile.Generate(minimalCtx("/home/me/proj"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{
		"COMPOSE_PROJECT_NAME=proj\n",
		"CONTAINER_SERVICE_NAME=dev\n",
		"USERNAME=developer\n",
		"IMAGE=ubuntu\n",
		"IMAGE_VERSION=24.04\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in .env, got:\n%s", want, body)
		}
	}
}

// TestGenerate_HeaderMarksHostIndependent pins the header claim that the
// file is safe to commit.
func TestGenerate_HeaderMarksHostIndependent(t *testing.T) {
	t.Parallel()

	body, err := envfile.Generate(minimalCtx("/home/me/proj"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(body, "# Host-independent: no UID/GID/docker-gid; safe to commit and share.\n") {
		t.Errorf("expected host-independent header, got:\n%s", body)
	}
}

// TestGenerate_NilContextReturnsSentinel verifies the entry guard returns a
// sentinel callers can match with errors.Is.
func TestGenerate_NilContextReturnsSentinel(t *testing.T) {
	t.Parallel()

	if _, err := envfile.Generate(nil); !errors.Is(err, envfile.ErrNilContext) {
		t.Errorf("Generate(nil) err = %v, want ErrNilContext", err)
	}
}
