package generate_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/generate"
)

// TestManageScript_NotEmpty pins the doc claim that ManageScript returns
// the embedded manage.sh contents.
func TestManageScript_NotEmpty(t *testing.T) {
	t.Parallel()
	if generate.ManageScript() == "" {
		t.Fatal("ManageScript() is empty; manage.sh embed failed")
	}
}

// TestManageScript_ShebangAndCommands pins that the embedded script is a
// bash script exposing the clean / rebuild / prune-cache commands and
// drives docker compose for project scoping.
func TestManageScript_ShebangAndCommands(t *testing.T) {
	t.Parallel()
	s := generate.ManageScript()
	if !strings.HasPrefix(s, "#!/usr/bin/env bash\n") {
		t.Error("manage.sh missing #!/usr/bin/env bash shebang")
	}
	for _, want := range []string{
		"clean", "rebuild", "prune-cache", "docker compose -f",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("manage.sh missing %q", want)
		}
	}
}

// TestManageScript_BashSyntax runs `bash -n` over the embedded manage.sh.
// The string-contains tests above cannot catch a parse error, which would
// otherwise only surface when a user runs ./.devcontainer/manage.sh.
func TestManageScript_BashSyntax(t *testing.T) {
	t.Parallel()
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not on PATH")
	}
	scriptPath := filepath.Join(t.TempDir(), "manage.sh")
	if writeErr := os.WriteFile(scriptPath, []byte(generate.ManageScript()), 0o600); writeErr != nil {
		t.Fatalf("write script: %v", writeErr)
	}
	cmd := exec.CommandContext(t.Context(), bashPath, "-n", scriptPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		t.Fatalf("bash -n rejected manage.sh: %v\n%s", runErr, stderr.String())
	}
}
