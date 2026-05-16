package generate_test

import (
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
