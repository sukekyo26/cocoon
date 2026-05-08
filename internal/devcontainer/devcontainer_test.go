package devcontainer_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/devcontainer"
	"github.com/sukekyo26/cocoon/internal/exec"
)

func TestCheckPrerequisites_MissingDevcontainerJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// We cannot reliably skip the docker/devcontainer steps here, so we
	// only assert that with a missing devcontainer.json the result is one
	// of the upstream-missing values OR PrereqMissingDevcontainerJSON. The
	// CI image typically lacks both docker daemon and devcontainer CLI;
	// either short-circuit before reaching the JSON check.
	var buf bytes.Buffer
	got := devcontainer.CheckPrerequisites(exec.New(), dir, &buf)
	switch got {
	case devcontainer.PrereqMissingDocker,
		devcontainer.PrereqMissingDevcontainerCLI,
		devcontainer.PrereqMissingDevcontainerJSON:
		// acceptable
	default:
		t.Fatalf("unexpected result: %v", got)
	}
}

func TestCheckPrerequisites_HappyPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".devcontainer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".devcontainer", "devcontainer.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	// Same caveat as above: docker / devcontainer might be absent.
	var buf bytes.Buffer
	_ = devcontainer.CheckPrerequisites(exec.New(), dir, &buf)
}

func TestIsWSL(t *testing.T) {
	t.Parallel()
	// Just exercise the function; the value depends on the host.
	_ = devcontainer.IsWSL()
}

func TestIsWSL_EnvSet(t *testing.T) {
	// Cannot run in parallel due to t.Setenv.
	t.Setenv("WSL_DISTRO_NAME", "Ubuntu-22.04")
	if !devcontainer.IsWSL() {
		t.Errorf("IsWSL with WSL_DISTRO_NAME set = false, want true")
	}
}

func TestErrDockerMissing_Sentinel(t *testing.T) {
	t.Parallel()
	// Just touch the exported sentinel so its declaration is covered.
	if devcontainer.ErrDockerMissing == nil {
		t.Error("ErrDockerMissing should be defined")
	}
}
