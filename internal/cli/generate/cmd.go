package generatecli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/logx"
)

const generateLong = `wsd generate-all — generate Dockerfile / docker-compose.yml / devcontainer.json

Loads <workspace.toml> and the enabled plugin TOMLs once, then writes every
generated artifact (Dockerfile, docker-compose.yml, devcontainer files,
the per-shell rc fragment) into <output_dir>.`

// NewCommand returns the cobra command for ` + "`wsd generate-all`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "generate-all <workspace.toml> <plugins_dir> <output_dir>",
		Short:         "Generate Dockerfile / docker-compose.yml / devcontainer.json",
		Long:          generateLong,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			log := logx.New(stdout, stderr)
			if len(args) != 3 {
				log.Error("Usage: wsd generate-all <workspace.toml> <plugins_dir> <output_dir>")
				return ErrUsage
			}
			ctx, err := loadContext(args[0], args[1], stderr)
			if err != nil {
				return err
			}
			arts, err := buildArtifacts(ctx, args[1], stderr)
			if err != nil {
				return err
			}
			if err := writeArtifacts(arts, args[2]); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	clihelpers.AttachHelpAlias(cmd)
	return cmd
}
