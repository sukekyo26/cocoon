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

// errCertsPathNotDirectory: caller wraps with ErrFailure once.
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
	if ctx.CertificatesEnabled() {
		if err := ensureUserCertsDir(stdout, cat); err != nil {
			return fmt.Errorf("%w: %w", ErrFailure, err)
		}
	}
	if ctx.HasHomeFiles() {
		if err := ensureHomeFiles(ctx, stdout, stderr, cat); err != nil {
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
	if ctx.HasHomeFiles() {
		printHomeFilesNotice(stdout, cat, ctx.HomeFilesEntries())
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

// ensureUserCertsDir mkdirs ~/.cocoon/certs (mode 0700) if missing so
// docker-compose's additional_contexts can resolve before the build
// starts. Only the first-run case prints a status line; re-runs stay
// quiet. Mirrors the plugin overlay's 0700 posture (private user data).
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

// printCertNotice surfaces `~/.cocoon/certs/` as the drop-zone for user
// CA certs in stdout, so users do not have to read configuration.md.
func printCertNotice(stdout io.Writer, cat *i18n.Catalog) {
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, cat.Msg("gen_certs_notice_header"))
	fmt.Fprintln(stdout, cat.Msg("gen_certs_notice_path"))
	fmt.Fprintln(stdout, cat.Msg("gen_certs_notice_team"))
}

// ensureHomeFiles touches each [home_files] entry on the host with mode
// 0600 so Docker does not auto-create them as directories at the first
// `docker compose up`. Existing files are left untouched (idempotent),
// directories collide with ErrHomeFileIsDirectory (callers see actionable
// recovery guidance in stderr), and symlinks are trusted as-is. When
// cocoon gen is running inside a container, the host is unreachable from
// here — emit a warning but still attempt the touch so contained dev
// loops where HOME happens to match still work.
func ensureHomeFiles(ctx *generate.WorkspaceContext, stdout, stderr io.Writer, cat *i18n.Catalog) error {
	files := ctx.HomeFilesEntries()
	if len(files) == 0 {
		return nil
	}
	if generate.InContainer() {
		fmt.Fprintln(stderr, cat.Msg("gen_home_files_in_container_warning"))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	for _, rel := range files {
		path := filepath.Join(home, rel)
		info, statErr := os.Lstat(path)
		switch {
		case statErr == nil && info.Mode()&os.ModeSymlink != 0:
			continue
		case statErr == nil && info.IsDir():
			fmt.Fprintln(stderr, cat.Msg("gen_home_files_is_directory", path, path))
			return fmt.Errorf("%s: %w", path, generate.ErrHomeFileIsDirectory)
		case statErr == nil:
			continue
		case !errors.Is(statErr, os.ErrNotExist):
			return fmt.Errorf("lstat %s: %w", path, statErr)
		}
		if mkErr := os.MkdirAll(filepath.Dir(path), 0o700); mkErr != nil {
			return fmt.Errorf("mkdir parent of %s: %w", path, mkErr)
		}
		f, openErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if openErr != nil {
			return fmt.Errorf("create %s: %w", path, openErr)
		}
		if closeErr := f.Close(); closeErr != nil {
			return fmt.Errorf("close %s: %w", path, closeErr)
		}
		fmt.Fprintln(stdout, cat.Msg("gen_home_file_touched", path))
	}
	return nil
}

// printHomeFilesNotice surfaces the configured [home_files] entries so
// users who skip VS Code Dev Containers (and therefore initializeCommand)
// can verify the host files exist before `docker compose up`.
func printHomeFilesNotice(stdout io.Writer, cat *i18n.Catalog, files []string) {
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, cat.Msg("gen_home_files_notice_header"))
	fmt.Fprintln(stdout, cat.Msg("gen_home_files_notice_check"))
	for _, rel := range files {
		fmt.Fprintln(stdout, cat.Msg("gen_home_files_notice_item", rel))
	}
}
