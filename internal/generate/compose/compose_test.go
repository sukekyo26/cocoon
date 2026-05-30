package compose_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/compose"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// updateGolden, when set with `go test -update-golden`, rewrites the
// testdata/*.expected files from the current generator output instead of
// asserting against them. Mirrors the dockerfile package convention.
//
//nolint:gochecknoglobals // test-only flag scoped to compose_test.
var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/*.expected from current generator output")

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
			plugins, err := plugin.LoadEnabledFromFS(os.DirFS(pluginsDir), ws.Plugins.Enable, &warns, pluginsDir)
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

			path := filepath.Join("testdata", tc.expectation)
			if *updateGolden {
				if werr := os.WriteFile(path, []byte(got), 0o600); werr != nil {
					t.Fatalf("update golden: %v", werr)
				}
				return
			}
			wantBytes, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read expected: %v", err)
			}
			if got != string(wantBytes) {
				t.Errorf("output mismatch (run with -update-golden to refresh)\n--- got ---\n%s\n--- want ---\n%s", got, string(wantBytes))
			}
		})
	}
}

// TestGenerate_CertificatesDisabled_NoAdditionalContexts verifies that
// when [certificates] is absent (or enable=false), the compose generator
// emits no `additional_contexts` mapping at all. Cert-free teams commit
// a compose file that has no cocoon_user_certs build context wiring.
func TestGenerate_CertificatesDisabled_NoAdditionalContexts(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	pluginsDir := filepath.Join(repoRoot, "internal", "plugin", "catalog")
	wsPath := filepath.Join(repoRoot, "tests", "fixtures", "snapshot.workspace.toml")

	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	ws.Certificates = nil

	var warns bytes.Buffer
	plugins, err := plugin.LoadEnabledFromFS(os.DirFS(pluginsDir), ws.Plugins.Enable, &warns, pluginsDir)
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

	for _, mustNot := range []string{
		"additional_contexts",
		"cocoon_user_certs",
		".cocoon/certs",
	} {
		if strings.Contains(got, mustNot) {
			t.Errorf("compose with [certificates] disabled must not contain %q\n--- got ---\n%s", mustNot, got)
		}
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

// TestGenerate_DockerSocketNoUserNoGroupAdd verifies that enabling
// docker_socket adds the socket bind mount but emits neither a `user:`
// override nor `group_add`: UID/GID and the docker group are resolved at
// container start by docker-entrypoint.sh, keeping the compose file
// host-independent.
func TestGenerate_DockerSocketNoUserNoGroupAdd(t *testing.T) {
	t.Parallel()

	socketOn := true
	ws := &config.Workspace{
		Container: config.ContainerSpec{
			ServiceName:  "dev",
			Username:     "dev",
			Image:        "ubuntu",
			ImageVersion: "24.04",
			DockerSocket: &socketOn,
		},
	}
	ctx := &generate.WorkspaceContext{WS: ws}
	got, err := compose.Generate(ctx, compose.Options{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(got, "/var/run/docker.sock:/var/run/docker.sock") {
		t.Errorf("docker_socket should still add the socket bind mount:\n%s", got)
	}
	for _, bad := range []string{"group_add", "${UID}", "${GID}", "${DOCKER_GID}"} {
		if strings.Contains(got, bad) {
			t.Errorf("host-independent compose must not contain %q:\n%s", bad, got)
		}
	}
}
