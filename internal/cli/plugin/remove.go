package plugincli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const removeLong = `cocoon plugin remove — delete a user / project overlay copy

The embedded catalog is never touched: --scope is required so the user is
always explicit about which overlay (user or project) is being deleted.
After removal, ` + "`cocoon plugin list`" + ` will show the plugin from the next
priority layer (or the embedded catalog).`

func newRemoveCmd(stdout, stderr io.Writer) *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:           "remove <id>",
		Short:         "Delete a user / project overlay copy of a plugin",
		Long:          removeLong,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return runRemove(stdout, stderr, args[0], scope)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "", `which overlay to delete: "user" or "project" (required)`)
	return cmd
}

func runRemove(stdout, _ io.Writer, id, scope string) error {
	if scope == "" {
		return fmt.Errorf("%w: --scope is required (use \"user\" or \"project\")", ErrUsage)
	}
	root, err := scopeDir(scope)
	if err != nil {
		return err
	}
	dst := filepath.Join(root, id)
	if _, statErr := os.Stat(dst); statErr != nil {
		if os.IsNotExist(statErr) {
			return fmt.Errorf("%w: %s does not exist", ErrUsage, dst)
		}
		return fmt.Errorf("%w: stat %s: %w", ErrFailure, dst, statErr)
	}
	if rmErr := os.RemoveAll(dst); rmErr != nil {
		return fmt.Errorf("%w: remove %s: %w", ErrFailure, dst, rmErr)
	}
	fmt.Fprintf(stdout, "Plugin %q removed from %s\n", id, dst)
	return nil
}
