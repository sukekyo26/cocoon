// Package gencli implements `cocoon gen`, the central generator command.
//
// `cocoon gen` discovers workspace.toml from the current directory (or
// an explicit --workspace path), assembles the layered plugin catalog
// (embedded < user < project), and writes Dockerfile,
// docker-compose.yml, and (when [workspace].devcontainer is true)
// devcontainer.json into .devcontainer/. Plugin install scripts are
// inlined directly into the generated Dockerfile so the build needs no
// external context beyond the project tree. Container start-up itself
// is left to the user — `docker compose -f .devcontainer/docker-compose.yml
// up -d` or VS Code's "Reopen in Container" both consume the output.
package gencli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	generatecli "github.com/sukekyo26/cocoon/internal/cli/generate"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure during generation.
var ErrFailure = errors.New("gen failed")

const genLong = `cocoon gen — generate .devcontainer/{Dockerfile, docker-compose.yml, devcontainer.json}

Discovers workspace.toml from the current directory (walking parent
directories until a .git boundary or $HOME), assembles the layered
plugin catalog (embedded < user < project), and writes the generated
artifacts under .devcontainer/. Plugin install scripts are inlined into
the generated Dockerfile, so the build needs no external context
beyond the project tree.

After generation, start the container yourself:

  docker compose -f .devcontainer/docker-compose.yml up -d

…or open the project in VS Code and pick "Reopen in Container".`

// NewCommand returns the cobra command for ` + "`cocoon gen`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	var (
		workspaceFlag string
		outputFlag    string
	)
	cmd := &cobra.Command{
		Use:           "gen",
		Short:         "Generate .devcontainer/ artifacts from workspace.toml",
		Long:          genLong,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runGen(stdout, stderr, workspaceFlag, outputFlag)
		},
	}
	cmd.Flags().StringVar(
		&workspaceFlag,
		"workspace",
		"",
		"path to workspace.toml (default: discovered from cwd)",
	)
	cmd.Flags().StringVar(
		&outputFlag,
		"output",
		"",
		"project root to write generated artifacts under (default: directory of workspace.toml)",
	)
	clihelpers.AttachHelpAlias(cmd)
	return cmd
}

func runGen(stdout, stderr io.Writer, workspaceFlag, outputFlag string) error {
	cat := i18n.New(i18n.Detect())
	wsPath, err := resolveWorkspace(workspaceFlag)
	if err != nil {
		return err
	}
	outDir := outputFlag
	if outDir == "" {
		outDir = filepath.Dir(wsPath)
	}

	catalog, err := plugin.CatalogFS()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailure, err)
	}
	userPluginDir, err := userPluginsDir()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailure, err)
	}
	projectPluginDir := filepath.Join(filepath.Dir(wsPath), ".cocoon", "plugins")
	layered := plugin.NewLayeredFS(catalog, userPluginDir, projectPluginDir)
	layered.LogOverrides(stderr)

	ctx, err := generatecli.LoadContext(wsPath, layered, "", stderr)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailure, err)
	}
	arts, err := generatecli.BuildArtifacts(ctx, stderr)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailure, err)
	}
	if err := generatecli.WriteArtifacts(arts, outDir); err != nil {
		return fmt.Errorf("%w: %w", ErrFailure, err)
	}

	cwd, _ := os.Getwd() //nolint:errcheck // cwd is best-effort for pretty-printing only.
	for _, a := range arts {
		fmt.Fprintln(stdout, cat.Msg("gen_wrote", displayPath(cwd, filepath.Join(outDir, a.Rel))))
	}
	printNextSteps(stdout, cat, ctx.WS.Workspace.DevContainerOrDefault())
	return nil
}

// displayPath shortens an absolute path to a cwd-relative form when the
// result stays inside cwd, so users see "wrote .devcontainer/Dockerfile"
// rather than "wrote /tmp/.../.devcontainer/Dockerfile".
func displayPath(cwd, p string) string {
	if cwd == "" {
		return p
	}
	rel, err := filepath.Rel(cwd, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p
	}
	return rel
}

func resolveWorkspace(flag string) (string, error) {
	if flag != "" {
		abs, err := filepath.Abs(flag)
		if err != nil {
			return "", fmt.Errorf("%w: resolve --workspace: %w", ErrUsage, err)
		}
		if _, statErr := os.Stat(abs); statErr != nil {
			return "", fmt.Errorf("%w: --workspace %s: %w", ErrUsage, abs, statErr)
		}
		return abs, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrFailure, err)
	}
	found, err := config.Discover(cwd)
	if err != nil {
		return "", fmt.Errorf("%w: discover workspace.toml: %w", ErrFailure, err)
	}
	if found == "" {
		return "", fmt.Errorf(
			"%w: workspace.toml not found in %s or any parent (try `cocoon init`)",
			ErrUsage, cwd,
		)
	}
	return found, nil
}

// userPluginsDir returns ~/.cocoon/plugins, the LayeredFS user layer root.
func userPluginsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".cocoon", "plugins"), nil
}

func printNextSteps(stdout io.Writer, cat *i18n.Catalog, devcontainer bool) {
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, cat.Msg("gen_next_header"))
	fmt.Fprintln(stdout, cat.Msg("gen_next_step_compose"))
	if devcontainer {
		fmt.Fprintln(stdout, cat.Msg("gen_next_step_vscode"))
	}
}
