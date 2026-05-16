package generate

import _ "embed"

// manageScript embeds manage.sh so cocoon ships as a single binary with
// no host-side script dependency.
//
//go:embed manage.sh
var manageScript string

// ManageScript returns the contents of .devcontainer/manage.sh, the
// project-scoped Docker clean / rebuild helper written next to the
// generated compose file with mode 0o755.
func ManageScript() string { return manageScript }
