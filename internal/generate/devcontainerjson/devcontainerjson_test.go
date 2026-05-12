package devcontainerjson_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/devcontainerjson"
)

// updateGolden, when set with `go test -update-golden`, rewrites the
// testdata/*.expected files from the current generator output instead of
// asserting against them. Mirrors the dockerfile package convention.
//
//nolint:gochecknoglobals // test-only flag scoped to devcontainerjson_test.
var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/*.expected from current generator output")

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
	path := filepath.Join("testdata", "snapshot.expected")
	if *updateGolden {
		if werr := os.WriteFile(path, []byte(got), 0o600); werr != nil {
			t.Fatalf("update golden: %v", werr)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch (run with -update-golden to refresh)\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}

// TestGenerateCertificatesDisabledNoInitializeCommand verifies that
// when [certificates] is absent (or enable=false), the devcontainer.json
// generator emits no `initializeCommand` key. Cert-free workspaces get
// no host-side mkdir hook in their VS Code dev container config.
func TestGenerateCertificatesDisabledNoInitializeCommand(t *testing.T) {
	t.Parallel()
	ws, err := config.LoadWorkspace(filepath.Join("..", "..", "..", "tests", "fixtures", "snapshot.workspace.toml"))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	ws.Certificates = nil

	ctx := &generate.WorkspaceContext{WS: ws}
	got, err := devcontainerjson.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	for _, mustNot := range []string{
		"initializeCommand",
		"mkdir -p",
		".cocoon/certs",
	} {
		if strings.Contains(got, mustNot) {
			t.Errorf("devcontainer.json with [certificates] disabled must not contain %q\n--- got ---\n%s", mustNot, got)
		}
	}
}

// TestGenerateHomeFilesAddsTouchToInitializeCommand verifies that a
// workspace with [home_files] but no [certificates] emits an
// initializeCommand made only of the per-file touch commands
// (umask 077 + touch). No certs mkdir, no leftover `&&` from a missing
// certs prefix.
func TestGenerateHomeFilesAddsTouchToInitializeCommand(t *testing.T) {
	t.Parallel()
	ctx := &generate.WorkspaceContext{
		WS: &config.Workspace{
			HomeFiles: &config.HomeFilesSpec{Files: []string{".claude.json"}},
		},
	}
	got, err := devcontainerjson.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, want := range []string{
		`"initializeCommand"`,
		`(umask 077 && mkdir -p \"$(dirname -- \"${HOME:?HOME must be set on the host}/.claude.json\")\" && touch \"${HOME:?HOME must be set on the host}/.claude.json\")`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output:\n%s", want, got)
		}
	}
	for _, mustNot := range []string{
		`mkdir -p \"${HOME:?HOME must be set on the host}/.cocoon/certs\"`,
	} {
		if strings.Contains(got, mustNot) {
			t.Errorf("home_files-only run must not emit %q:\n%s", mustNot, got)
		}
	}
}

// TestGenerateHomeFilesAndCertsMergeInitializeCommand verifies that when
// both [certificates] and [home_files] are configured, the generator
// emits a single initializeCommand key with `&&`-joined steps (certs
// mkdir first, then each home_files touch in declaration order). The
// orderedMap must not contain duplicate keys.
func TestGenerateHomeFilesAndCertsMergeInitializeCommand(t *testing.T) {
	t.Parallel()
	certsOn := true
	ctx := &generate.WorkspaceContext{
		WS: &config.Workspace{
			Certificates: &config.CertificatesSpec{Enable: &certsOn},
			HomeFiles:    &config.HomeFilesSpec{Files: []string{".claude.json", ".gemini/oauth_creds.json"}},
		},
	}
	got, err := devcontainerjson.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	wantLine := `"initializeCommand": "mkdir -p \"${HOME:?HOME must be set on the host}/.cocoon/certs\" && (umask 077 && mkdir -p \"$(dirname -- \"${HOME:?HOME must be set on the host}/.claude.json\")\" && touch \"${HOME:?HOME must be set on the host}/.claude.json\") && (umask 077 && mkdir -p \"$(dirname -- \"${HOME:?HOME must be set on the host}/.gemini/oauth_creds.json\")\" && touch \"${HOME:?HOME must be set on the host}/.gemini/oauth_creds.json\")"`
	if !strings.Contains(got, wantLine) {
		t.Errorf("merged initializeCommand mismatch.\nwant line: %s\n--- got ---\n%s", wantLine, got)
	}
	if n := strings.Count(got, `"initializeCommand"`); n != 1 {
		t.Errorf("initializeCommand should appear exactly once, got %d:\n%s", n, got)
	}
}

// TestGenerateBothCertsAndHomeFilesDisabledOmitsInitializeCommand pins
// that the initializeCommand key is omitted entirely when neither
// [certificates] nor [home_files] is configured. Pairs with the existing
// TestGenerateCertificatesDisabledNoInitializeCommand which exercises
// the same omission for the certs-only branch.
func TestGenerateBothCertsAndHomeFilesDisabledOmitsInitializeCommand(t *testing.T) {
	t.Parallel()
	ctx := &generate.WorkspaceContext{WS: &config.Workspace{}}
	got, err := devcontainerjson.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if strings.Contains(got, "initializeCommand") {
		t.Errorf("initializeCommand must not appear when no host hooks are configured:\n%s", got)
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
		`"workspaceFolder": "/home/developer/workspace/dev"`,
		`"shutdownAction": "stopCompose"`,
		`"extensions": []`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output:\n%s", want, got)
		}
	}
	// With no [ports] section and no devcontainer.forward_ports override,
	// the forwardPorts key must be omitted entirely — emitting [3000]
	// (the historic default) silently forwarded a port the user never
	// declared, polluting `docker compose ps` and VS Code's "Ports" panel.
	if strings.Contains(got, `"forwardPorts"`) {
		t.Errorf("forwardPorts should be omitted when no ports are declared, got:\n%s", got)
	}
}

// TestGenerateForwardPortsFromOverrideOnly verifies the key is emitted
// when devcontainer.forward_ports supplies values but [ports] is absent —
// the override path is the sole source.
func TestGenerateForwardPortsFromOverrideOnly(t *testing.T) {
	t.Parallel()
	ctx := &generate.WorkspaceContext{
		WS: &config.Workspace{
			Devcontainer: config.Devcontainer{
				"forwardPorts": []any{int64(9000)},
			},
		},
	}
	got, err := devcontainerjson.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(got, "\"forwardPorts\": [\n\t\t9000\n\t]") {
		t.Errorf("expected forwardPorts=[9000] from override, got:\n%s", got)
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
