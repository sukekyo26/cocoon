package verifyartifactscli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	generatecli "github.com/sukekyo26/cocoon/internal/cli/generate"
	verifyartifactscli "github.com/sukekyo26/cocoon/internal/cli/verifyartifacts"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(here), "..", "..", ".."))
}

//nolint:unparam // test helper returns both buffers; not all callers use both
func runGenerate(args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := generatecli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

//nolint:unparam // test helper returns both buffers; not all callers use both
func runVerify(args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := verifyartifactscli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

func TestVerifyArtifacts_AllCIFixturesPass(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	pluginsDir := filepath.Join(root, "plugins")

	for _, name := range []string{"pinned", "latest", "arm64-smoke"} {
		fixture := filepath.Join(root, "tests", "fixtures", "ci", name+".workspace.toml")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			outDir := t.TempDir()

			_, genStderr, err := runGenerate(fixture, pluginsDir, outDir)
			if err != nil {
				t.Fatalf("generate-all failed: %v\nstderr:\n%s", err, genStderr.String())
			}

			_, stderr, err := runVerify(fixture, outDir)
			if err != nil {
				t.Fatalf("verify-artifacts failed: %v\nstderr:\n%s", err, stderr.String())
			}
		})
	}
}

func TestVerifyArtifacts_DetectsMissingResource(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	pluginsDir := filepath.Join(root, "plugins")
	fixture := filepath.Join(root, "tests", "fixtures", "ci", "pinned.workspace.toml")

	outDir := t.TempDir()
	if _, _, err := runGenerate(fixture, pluginsDir, outDir); err != nil {
		t.Fatalf("generate-all failed: %v", err)
	}

	composePath := filepath.Join(outDir, "docker-compose.yml")
	body, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	tampered := bytes.ReplaceAll(body, []byte("shm_size: 2gb"), []byte("# removed"))
	if bytes.Equal(tampered, body) {
		t.Fatal("expected shm_size: 2gb to appear in compose output")
	}
	if writeErr := os.WriteFile(composePath, tampered, 0o600); writeErr != nil { //nolint:gosec // composePath is t.TempDir()-rooted in this test.
		t.Fatalf("write compose: %v", writeErr)
	}

	_, stderr, err := runVerify(fixture, outDir)
	if !errors.Is(err, verifyartifactscli.ErrFailure) {
		t.Fatalf("expected ErrFailure, got %v\nstderr:\n%s", err, stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("compose.shm_size")) {
		t.Fatalf("expected compose.shm_size failure in stderr, got:\n%s", stderr.String())
	}
}

func TestVerifyArtifacts_UsageError(t *testing.T) {
	t.Parallel()

	_, _, err := runVerify()
	if !errors.Is(err, verifyartifactscli.ErrUsage) {
		t.Fatalf("expected ErrUsage, got %v", err)
	}
}
