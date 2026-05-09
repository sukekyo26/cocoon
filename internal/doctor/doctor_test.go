package doctor_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/doctor"
)

func TestReporter_Counters(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := doctor.NewReporter(&buf)
	r.Pass("p")
	r.Warn("w", "hint")
	r.Fail("f", "fix")
	r.Summary()
	out := buf.String()
	for _, sub := range []string{"[✓]", " p\n", "[⚠]", " w\n", "→ hint", "[✗]", " f\n", "→ fix", "Summary: 1 passed, 1 warning, 1 failed"} {
		if !strings.Contains(out, sub) {
			t.Errorf("missing %q in:\n%s", sub, out)
		}
	}
	if !r.HasFailures() {
		t.Error("HasFailures should be true")
	}
}

// TestRun_MinimalRoot exercises the orchestrator on a workspace where most
// optional artefacts are absent. It does not assert exact byte-for-byte
// output (host-dependent: docker, uv, bash version) but verifies the
// banner, that the workspace.toml check passes, and that the summary line
// is present.
func TestRun_MinimalRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	body := `[container]
service_name = "dev"
username = "dev"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = []
`
	if err := os.WriteFile(filepath.Join(root, "workspace.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	doctor.Run(doctor.Options{Root: root}, &buf)
	out := buf.String()
	for _, sub := range []string{
		"workspace-docker doctor",
		"========================",
		"workspace.toml exists",
		"Summary:",
	} {
		if !strings.Contains(out, sub) {
			t.Errorf("missing %q in output:\n%s", sub, out)
		}
	}
}

// TestRun_RichRoot drives the orchestrator with most optional artefacts
// present so the per-check branches (compose syntax, init guard, dockerfile
// buildkit, sidecars, repositories, certificates) all execute. We don't
// assert exact pass/fail counts because those depend on whether docker is
// reachable on the host; we only verify the orchestrator runs to completion
// without panicking.
func TestRun_RichRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	body := `[container]
service_name = "app"
username = "dev"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = []

[services.db]
image = "postgres:16"

[repositories]
clone = []
`
	if err := os.WriteFile(filepath.Join(root, "workspace.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	// Drop dummy artefacts so the corresponding checks have files to inspect.
	if err := os.WriteFile(filepath.Join(root, "Dockerfile"),
		[]byte("# syntax=docker/dockerfile:1\nFROM ubuntu:24.04\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docker-compose.yml"),
		[]byte("services:\n  app:\n    image: ubuntu:24.04\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "certs"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	doctor.Run(doctor.Options{Root: root, PluginsDir: filepath.Join(root, "internal", "plugin", "catalog")}, &buf)
	out := buf.String()
	if !strings.Contains(out, "Summary:") {
		t.Errorf("missing Summary line:\n%s", out)
	}
}
