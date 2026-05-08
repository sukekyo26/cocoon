package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/version"
)

func main() {
	root := &cobra.Command{
		Use:           "cocoon",
		Short:         "Project-aware container workspace generator",
		Long:          "Generate Dev Containers and docker-compose stacks tailored to each project from a single workspace.toml.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(versionCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print cocoon version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(version.Get())
		},
	}
}
