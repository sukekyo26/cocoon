//nolint:testpackage
package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
)

func TestWriteWorkspaceToml(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "wsd-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	wsPath := filepath.Join(tmpDir, "workspace.toml")
	ws := &config.Workspace{
		Container: config.ContainerSpec{
			ServiceName: "test-svc",
			Username:    "test-user",
			Os:          "ubuntu",
			OsVersion:   "24.04",
		},
		Plugins: config.PluginsSpec{Enable: []string{"go", "rust"}},
		Ports: &config.PortsSpec{Forward: []any{
			"8080:8080",
			map[string]any{"target": int64(5432), "host_ip": "127.0.0.1", "protocol": "tcp"},
		}},
		Volumes: map[string]string{"/data": "volume1"},
	}

	if writeErr := writeWorkspaceToml(wsPath, ws); writeErr != nil {
		t.Fatalf("failed to write: %v", writeErr)
	}

	data, err := os.ReadFile(wsPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Check for key sections and values
	checks := []string{
		"[container]",
		"service_name = \"test-svc\"",
		"username = \"test-user\"",
		"[plugins]",
		"enable = [\"go\", \"rust\"]",
		"[ports]",
		`forward = ["8080:8080", { target = 5432, host_ip = "127.0.0.1", protocol = "tcp" }]`,
		"[volumes]",
		"/data = \"volume1\"",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("expected content to contain %q", check)
		}
	}
}

func TestWriteEnv(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "wsd-test-env-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	ws := &config.Workspace{
		Container: config.ContainerSpec{
			ServiceName: "test-svc",
			Username:    "test-user",
			Os:          "ubuntu",
			OsVersion:   "24.04",
		},
	}

	if writeErr := writeEnv(tmpDir, ws, 1000, 1000, 999); writeErr != nil {
		t.Fatalf("failed to write: %v", writeErr)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	checks := []string{
		"CONTAINER_SERVICE_NAME=test-svc",
		"USERNAME=test-user",
		"UID=1000",
		"GID=1000",
		"DOCKER_GID=999",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("expected .env to contain %q", check)
		}
	}
}

func TestRenderEnvGolden(t *testing.T) {
	t.Parallel()
	ws := &config.Workspace{
		Container: config.ContainerSpec{
			ServiceName: "dev",
			Username:    "shogo",
			Os:          "ubuntu",
			OsVersion:   "24.04",
		},
	}
	got, err := renderEnv("/home/shogo/workspace/myproj", ws, 1000, 1000, 999)
	if err != nil {
		t.Fatalf("renderEnv: %v", err)
	}
	want := `# Environment variables for docker-compose
# Auto-generated from workspace.toml — do not edit manually
# Regenerate with: ./setup-docker.sh

COMPOSE_PROJECT_NAME=myproj
CONTAINER_SERVICE_NAME=dev
USERNAME=shogo
UID=1000
GID=1000
DOCKER_GID=999
OS_IMAGE=ubuntu
OS_VERSION=24.04
`
	if got != want {
		t.Errorf(".env mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
