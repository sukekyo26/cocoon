package schema

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"
)

const schemaLong = `wsd schema — generate the workspace.toml JSON Schema

Generates a JSON Schema describing every accepted field in workspace.toml.
The output is consumed by editors (VS Code, IntelliJ) and by ` + "`just schema-check`" + `.`

// NewCommand returns the cobra subtree for ` + "`wsd schema`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "schema",
		Short:         "Generate the workspace.toml JSON Schema",
		Long:          schemaLong,
		Args:          rejectUnknownSubcommand,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Legacy behaviour: `wsd schema` with no subcommand prints usage
			// to stderr and exits 2 (ErrUsage). Redirect cobra's help writer
			// to stderr for this error path; `--help` still routes to stdout
			// because it bypasses RunE.
			cmd.SetOut(stderr)
			if err := cmd.Help(); err != nil {
				return err //nolint:wrapcheck // help write error is descriptive
			}
			return ErrUsage
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	cmd.AddCommand(newDumpCmd(stdout))
	return cmd
}

// rejectUnknownSubcommand returns an ErrUsage-wrapped error when a stray
// positional appears under a parent that only carries subcommands.
func rejectUnknownSubcommand(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return fmt.Errorf("%w: unknown subcommand %q", ErrUsage, args[0])
}

func newDumpCmd(stdout io.Writer) *cobra.Command {
	var (
		write  bool
		output string
	)
	cmd := &cobra.Command{
		Use:           "dump",
		Short:         "Print or write the workspace.toml JSON Schema",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			data, err := Generate()
			if err != nil {
				return fmt.Errorf("%w: %w", ErrFailure, err)
			}
			switch {
			case output != "":
				return writeFile(output, data)
			case write:
				return writeFile(filepath.Join("schemas", "workspace.schema.json"), data)
			default:
				if _, werr := stdout.Write(data); werr != nil {
					return fmt.Errorf("%w: %w", ErrFailure, werr)
				}
				return nil
			}
		},
	}
	cmd.Flags().BoolVar(&write, "write", false, "write schemas/workspace.schema.json")
	cmd.Flags().StringVar(&output, "output", "", "write schema to given path")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	return cmd
}
