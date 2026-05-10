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
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure during generation.
var ErrFailure = errors.New("gen failed")

// errCertsPathNotDirectory is returned by ensureUserCertsDir when the
// target ~/.cocoon/certs path exists but is not a directory (a stray
// regular file, a broken symlink, etc.). Kept package-private because
// it is not part of the public surface — callers map runtime failures
// to ErrFailure at the wrap site.
var errCertsPathNotDirectory = errors.New("cocoon certs path exists but is not a directory")

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
	// Host-side cert directory + notice only land when the workspace
	// opts into [certificates] enable=true. Teams that never touch certs
	// get neither the side effect (mkdir on $HOME) nor the notice.
	if ctx.CertificatesEnabled() {
		if err := ensureUserCertsDir(stdout, cat); err != nil {
			return fmt.Errorf("%w: %w", ErrFailure, err)
		}
	}

	cwd, _ := os.Getwd() //nolint:errcheck // cwd is best-effort for pretty-printing only.
	for _, a := range arts {
		fmt.Fprintln(stdout, cat.Msg("gen_wrote", displayPath(cwd, filepath.Join(outDir, a.Rel))))
	}
	printNextSteps(stdout, cat, ctx.WS.Workspace.DevContainerOrDefault())
	if ctx.CertificatesEnabled() {
		printCertNotice(stdout, cat)
	}
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

// ensureUserCertsDir creates ~/.cocoon/certs (mode 0700) if it does not
// exist yet. The path is referenced by the generated docker-compose.yml's
// additional_contexts (cocoon_user_certs -> ${HOME}/.cocoon/certs);
// BuildKit requires the source path to exist before the build starts.
//
// VS Code Dev Containers users get this auto-created via the generated
// devcontainer.json's initializeCommand, but anyone running
// `docker compose build` directly (CI, terminal-only flows) needs the
// directory present. Doing it here in `cocoon gen` removes one manual
// step for the developer who runs the generator. A status line is
// emitted to stdout only when the directory was actually created (i.e.
// did not exist before) so re-running gen on an established workspace
// stays quiet.
//
// Permission is 0700 to mirror the security posture of plugin overlays
// at ~/.cocoon/plugins/ (private user data, never committed to a repo).
func ensureUserCertsDir(stdout io.Writer, cat *i18n.Catalog) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	dir := filepath.Join(home, generate.CertsHostPathRelative)
	info, err := os.Stat(dir)
	switch {
	case err == nil && info.IsDir():
		return nil
	case err == nil && !info.IsDir():
		// A non-directory at the cert path (a stray file, a symlink, etc.)
		// would silently break BuildKit's additional_contexts resolution
		// at build time. Surface it as a plain error; the caller wraps
		// once with ErrFailure so we do not double-wrap and produce a
		// `gen failed: gen failed: …` user-facing prefix.
		return fmt.Errorf("%s: %w", dir, errCertsPathNotDirectory)
	case !os.IsNotExist(err):
		return fmt.Errorf("stat %s: %w", dir, err)
	}
	if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
		return fmt.Errorf("mkdir %s: %w", dir, mkErr)
	}
	fmt.Fprintln(stdout, cat.Msg("gen_certs_dir_created", dir))
	return nil
}

func printNextSteps(stdout io.Writer, cat *i18n.Catalog, devcontainer bool) {
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, cat.Msg("gen_next_header"))
	fmt.Fprintln(stdout, cat.Msg("gen_next_step_compose"))
	if devcontainer {
		fmt.Fprintln(stdout, cat.Msg("gen_next_step_vscode"))
	}
}

// printCertNotice writes a short informational block telling the user
// that ~/.cocoon/certs is the host-side drop-zone for user CA certs.
// The block is emitted unconditionally because every team scenario
// touches the directory in some form (Dockerfile/compose/devcontainer
// all reference it), and surfacing it once per `cocoon gen` keeps the
// expectation visible without forcing the user to read configuration.md.
func printCertNotice(stdout io.Writer, cat *i18n.Catalog) {
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, cat.Msg("gen_certs_notice_header"))
	fmt.Fprintln(stdout, cat.Msg("gen_certs_notice_path"))
	fmt.Fprintln(stdout, cat.Msg("gen_certs_notice_team"))
}
