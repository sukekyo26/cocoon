// Package gencli implements `cocoon gen`, an alias for the v1
// `generate-all` subcommand. The implementation simply re-exports the
// existing internal/cli/generate command tree under a shorter name; the
// command-tree reshuffle in F3 will make `gen` the canonical form and
// retire `generate-all`.
package gencli

import (
	"io"

	"github.com/spf13/cobra"

	generatecli "github.com/sukekyo26/cocoon/internal/cli/generate"
)

// NewCommand returns the `cocoon gen` cobra command. It reuses the
// existing generate-all RunE, so flags, error sentinels and validation
// stay in lock-step with the legacy command until F3 collapses them.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := generatecli.NewCommand(stdout, stderr)
	cmd.Use = "gen"
	cmd.Short = "Generate Dockerfile / docker-compose.yml / devcontainer.json (alias of generate-all)"
	return cmd
}
