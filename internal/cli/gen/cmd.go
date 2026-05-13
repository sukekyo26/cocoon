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
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure during generation.
var ErrFailure = errors.New("gen failed")

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

// NewCommand returns the cobra command for `cocoon gen`.
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
	log := logx.New(stdout, stderr)
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
		if err := ensureUserCertsDir(log, cat); err != nil {
			return fmt.Errorf("%w: %w", ErrFailure, err)
		}
	}
	if ctx.HasHomeFiles() {
		if err := ensureHomeFiles(ctx, log, cat); err != nil {
			return fmt.Errorf("%w: %w", ErrFailure, err)
		}
	}

	cwd, _ := os.Getwd() //nolint:errcheck // cwd is best-effort for pretty-printing only.
	for _, a := range arts {
		log.Success(cat.Msg("gen_wrote", displayPath(cwd, filepath.Join(outDir, a.Rel))))
	}
	printNextSteps(log, cat, ctx.WS.Workspace.DevContainerOrDefault())
	if ctx.CertificatesEnabled() {
		printCertNotice(log, cat)
	}
	if ctx.HasHomeFiles() {
		printHomeFilesNotice(log, cat, ctx.HomeFilesEntries())
	}
	return nil
}

// displayPath returns a cwd-relative form when the result stays inside cwd.
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

// userPluginsDir is the LayeredFS user-layer root (~/.cocoon/plugins).
func userPluginsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".cocoon", "plugins"), nil
}

// ensureUserCertsDir mkdirs ~/.cocoon/certs (0700) so additional_contexts
// resolves before the build starts. Idempotent: re-runs stay quiet.
func ensureUserCertsDir(log *logx.Logger, cat *i18n.Catalog) error {
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
	log.Success(cat.Msg("gen_certs_dir_created", dir))
	return nil
}

func printNextSteps(log *logx.Logger, cat *i18n.Catalog, devcontainer bool) {
	log.Info("")
	log.Info(log.Bold(cat.Msg("gen_next_header")))
	log.Info(cat.Msg("gen_next_step_compose"))
	if devcontainer {
		log.Info(cat.Msg("gen_next_step_vscode"))
	}
}

// printCertNotice surfaces ~/.cocoon/certs/ as the user-CA drop zone.
func printCertNotice(log *logx.Logger, cat *i18n.Catalog) {
	log.Info("")
	log.Info(log.Bold(cat.Msg("gen_certs_notice_header")))
	log.Info(cat.Msg("gen_certs_notice_path"))
	log.Info(cat.Msg("gen_certs_notice_team"))
}

// ensureHomeFiles touches each [home_files] entry (mode 0600) so Docker
// does not auto-create them as directories. Idempotent on regular files;
// directories collide with ErrHomeFileIsDirectory; symlinks are trusted.
// When run inside a container the host is unreachable, so we warn but
// still try (contained loops where HOME matches still work).
func ensureHomeFiles(ctx *generate.WorkspaceContext, log *logx.Logger, cat *i18n.Catalog) error {
	files := ctx.HomeFilesEntries()
	if len(files) == 0 {
		return nil
	}
	if generate.InContainer() {
		log.Warn(cat.Msg("gen_home_files_in_container_warning"))
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
			log.Warn(cat.Msg("gen_home_files_is_directory", path, path))
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
		if errors.Is(openErr, os.ErrExist) {
			// Lost a TOCTOU race against another writer; the desired
			// state (file exists) is already satisfied.
			continue
		}
		if openErr != nil {
			return fmt.Errorf("create %s: %w", path, openErr)
		}
		if closeErr := f.Close(); closeErr != nil {
			return fmt.Errorf("close %s: %w", path, closeErr)
		}
		log.Success(cat.Msg("gen_home_file_touched", path))
	}
	return nil
}

// printHomeFilesNotice surfaces the configured [home_files] entries so
// users who skip VS Code Dev Containers (and therefore initializeCommand)
// can verify the host files exist before `docker compose up`.
func printHomeFilesNotice(log *logx.Logger, cat *i18n.Catalog, files []string) {
	log.Info("")
	log.Info(log.Bold(cat.Msg("gen_home_files_notice_header")))
	log.Info(cat.Msg("gen_home_files_notice_check"))
	for _, rel := range files {
		log.Info(cat.Msg("gen_home_files_notice_item", rel))
	}
}
