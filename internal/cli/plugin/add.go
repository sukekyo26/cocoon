package plugincli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

const addLong = `cocoon plugin add — copy an embedded plugin into your overlay so you can edit it

The copy lands at:

  --scope user (default):  ~/.cocoon/plugins/<id>/
  --scope project:         <project>/.cocoon/plugins/<id>/   (workspace.toml must be discoverable)

Once present, the overlay wins over the embedded catalog (project > user >
embedded). To revert to the embedded version, ` + "`cocoon plugin remove <id>`" + `.

Refuses to overwrite an existing target unless --force is passed.`

func newAddCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		scope string
		force bool
	)
	cmd := &cobra.Command{
		Use:           "add <id>",
		Short:         "Copy an embedded plugin into the user / project overlay",
		Long:          addLong,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return runAdd(stdout, stderr, args[0], scope, force)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", plugin.SourceUser, `where to copy: "user" or "project"`)
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing overlay copy")
	return cmd
}

func runAdd(stdout, _ io.Writer, id, scope string, force bool) error {
	embed, err := plugin.CatalogFS()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailure, err)
	}
	manifest, ferr := embed.Open(id + "/plugin.toml")
	if ferr != nil {
		return fmt.Errorf("%w: plugin %q is not in the embedded catalog", ErrUsage, id)
	}
	_ = manifest.Close()

	root, err := scopeDir(scope)
	if err != nil {
		return err
	}
	dst := filepath.Join(root, id)
	if info, statErr := os.Stat(dst); statErr == nil {
		if !info.IsDir() {
			return fmt.Errorf("%w: %s exists and is not a directory", ErrFailure, dst)
		}
		if !force {
			return fmt.Errorf("%w: %s already exists; pass --force to overwrite", ErrUsage, dst)
		}
	}

	// Materialize handles the "remove the existing <dst>/<id>/ first, copy
	// fresh, chmod *.sh to 0o755" semantics we want here.
	if matErr := plugin.Materialize(embed, []string{id}, root); matErr != nil {
		return fmt.Errorf("%w: copy %q to %s: %w", ErrFailure, id, dst, matErr)
	}
	fmt.Fprintf(stdout, "Plugin %q copied to %s (%s overlay)\n", id, dst, scope)
	return nil
}
