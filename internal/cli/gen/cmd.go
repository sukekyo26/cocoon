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
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	generatecli "github.com/sukekyo26/cocoon/internal/cli/generate"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/fsx"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/lockfile"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

var errCertsPathNotDirectory = errors.New("cocoon certs path exists but is not a directory")

// dockerCLIPluginID is the catalog id of the Docker CLI plugin. Enabling it
// without docker_socket leaves the in-container client with no daemon to
// reach; warnDockerCLIWithoutSocket flags that combination.
const dockerCLIPluginID = "docker-cli"

func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cat := i18n.New(i18n.Detect())
	var (
		workspaceFlag string
		outputFlag    string
		locked        bool
	)
	cmd := &cobra.Command{
		Use:           "gen",
		Short:         cat.Msg("cmd_gen_short"),
		Long:          cat.Msg("cmd_gen_long"),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runGen(stdout, stderr, workspaceFlag, outputFlag, locked)
		},
	}
	cmd.Flags().StringVar(&workspaceFlag, "workspace", "", cat.Msg("flag_gen_workspace_usage"))
	cmd.Flags().StringVar(&outputFlag, "output", "", cat.Msg("flag_gen_output_usage"))
	cmd.Flags().BoolVar(&locked, "locked", false, cat.Msg("flag_gen_locked_usage"))
	clihelpers.AttachHelpAlias(cmd)
	cmd.AddCommand(newWorkspaceCmd(stdout, stderr))
	return cmd
}

// loadGenContext resolves workspace.toml from workspaceFlag (falling back
// to discovery from cwd), determines the output directory (defaulting to
// the workspace.toml directory), assembles the layered plugin FS
// (embedded < user < project), and returns a loaded WorkspaceContext.
// Shared by `cocoon gen` and `cocoon gen workspace` so the discovery
// rules stay in lockstep.
func loadGenContext(stderr io.Writer, workspaceFlag, outputFlag string) (
	outDir string,
	ctx *generate.WorkspaceContext,
	err error,
) {
	wsPath, err := resolveWorkspace(workspaceFlag)
	if err != nil {
		return "", nil, err
	}
	outDir = outputFlag
	if outDir == "" {
		outDir = filepath.Dir(wsPath)
	}
	// Normalize to an absolute path so downstream callers can mix it with
	// abs paths (filepath.Rel rejects relative + absolute) and display
	// helpers stay deterministic regardless of cwd-at-print-time.
	outDir, err = filepath.Abs(outDir)
	if err != nil {
		return "", nil, fmt.Errorf("%w: resolve --output: %w", clihelpers.ErrUsage, err)
	}
	catalog, err := plugin.CatalogFS()
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
	userPluginDir, err := userPluginsDir()
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
	projectPluginDir := filepath.Join(filepath.Dir(wsPath), ".cocoon", "plugins")
	layered := plugin.NewLayeredFS(catalog, userPluginDir, projectPluginDir)
	layered.LogOverrides(stderr)

	ctx, err = generatecli.LoadContext(wsPath, layered, "", stderr)
	if err != nil {
		// Returned as-is: generatecli attaches ErrFailure, so re-wrapping
		// would double the "failure:" prefix (defensive-coding §3).
		return "", nil, err //nolint:wrapcheck // ErrFailure already attached by generatecli
	}
	// gen consumes cocoon.lock (when present) for reproducible PIN/CHECKSUM_*;
	// a malformed lock is a hard failure here (unlike `cocoon lock`, which
	// overwrites it).
	lock, lockErr := lockfile.Load(lockfile.PathFor(wsPath, ctx.WS))
	if lockErr != nil && !lockfile.IsNotExist(lockErr) {
		return "", nil, fmt.Errorf("%w: %w", clihelpers.ErrFailure, lockErr)
	}
	ctx.Lock = lock // nil when absent
	return outDir, ctx, nil
}

func runGen(stdout, stderr io.Writer, workspaceFlag, outputFlag string, locked bool) error {
	cat := i18n.New(i18n.Detect())
	log := logx.New(stdout, stderr)
	outDir, ctx, err := loadGenContext(stderr, workspaceFlag, outputFlag)
	if err != nil {
		return err
	}
	if lockErr := enforceLocked(ctx, locked, log, cat); lockErr != nil {
		return lockErr
	}
	arts, err := generatecli.BuildArtifacts(ctx, stderr)
	if err != nil {
		return err //nolint:wrapcheck // ErrFailure already attached by generatecli
	}
	if err := generatecli.WriteArtifacts(arts, outDir); err != nil {
		return err //nolint:wrapcheck // ErrFailure already attached by generatecli
	}
	if ctx.CertificatesEnabled() {
		if err := ensureUserCertsDir(log, cat); err != nil {
			return fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
		}
	}
	if ctx.HasHomeFiles() {
		if err := ensureHomeFiles(ctx, log, cat); err != nil {
			return fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
		}
	}
	if err := ensureSudoGitignore(ctx, outDir, log, cat); err != nil {
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}

	warnDockerCLIWithoutSocket(ctx, log, cat)
	warnPasswordSudoMissingSecret(ctx, outDir, log, cat)

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
			return "", fmt.Errorf("%w: resolve --workspace: %w", clihelpers.ErrUsage, err)
		}
		if _, statErr := os.Stat(abs); statErr != nil {
			return "", fmt.Errorf("%w: --workspace %s: %w", clihelpers.ErrUsage, abs, statErr)
		}
		return abs, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
	found, err := config.Discover(cwd)
	if err != nil {
		return "", fmt.Errorf("%w: discover workspace.toml: %w", clihelpers.ErrFailure, err)
	}
	if found == "" {
		return "", fmt.Errorf(
			"%w: workspace.toml not found in %s or any parent (try `cocoon init`)",
			clihelpers.ErrUsage, cwd,
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
	log.Info(cat.Msg("gen_next_step_manage"))
}

// warnDockerCLIWithoutSocket flags the docker-cli-without-docker_socket
// combination: the in-container docker client has no daemon socket to reach,
// so `docker ...` fails at runtime. Only a warning — using docker-cli against
// a remote DOCKER_HOST is a legitimate socket-less setup.
// enforceLocked applies the cocoon.lock reproducibility policy: any enabled
// plugin whose "latest" constraint has no lock entry resolves
// non-reproducibly at build time. Without --locked each is a warning; with
// --locked the set is a usage error pointing at `cocoon lock`.
func enforceLocked(ctx *generate.WorkspaceContext, locked bool, log *logx.Logger, cat *i18n.Catalog) error {
	unlocked := ctx.UnlockedLatestPlugins()
	if len(unlocked) == 0 {
		return nil
	}
	if locked {
		return fmt.Errorf(
			`%w: plugins use "latest" without a cocoon.lock entry: %s; run `+"`cocoon lock`"+` (or drop --locked)`,
			clihelpers.ErrUsage, strings.Join(unlocked, ", "))
	}
	for _, id := range unlocked {
		log.Warn(cat.Msg("gen_unlocked_latest_warning", id))
	}
	return nil
}

func warnDockerCLIWithoutSocket(ctx *generate.WorkspaceContext, log *logx.Logger, cat *i18n.Catalog) {
	if slices.Contains(ctx.EnabledPlugins(), dockerCLIPluginID) &&
		!ctx.WS.Container.DockerSocketEnabled() {
		log.Warn(cat.Msg("gen_docker_cli_without_socket_warning"))
	}
}

// ensureSudoGitignore upserts the .env.local ignore line into
// .devcontainer/.gitignore when password sudo is enabled, preserving any
// existing user rules (it appends rather than overwriting, so a user-managed
// .gitignore is never clobbered). No-op when the line is already present or
// password sudo is off.
func ensureSudoGitignore(
	ctx *generate.WorkspaceContext, outDir string, log *logx.Logger, cat *i18n.Catalog,
) error {
	if !ctx.PasswordSudoEnabled() {
		return nil
	}
	path := filepath.Join(outDir, ".devcontainer", ".gitignore")
	changed, err := fsx.EnsureGitignoreEntry(
		path, generate.SudoPasswordSecretFile, generate.SudoPasswordGitignoreComment)
	if err != nil {
		return err //nolint:wrapcheck // runGen attaches ErrFailure once.
	}
	if changed {
		log.Success(cat.Msg("gen_sudo_gitignore_ensured", path))
	}
	return nil
}

// warnPasswordSudoMissingSecret flags password sudo enabled without a usable
// .env.local secret file — absent or empty (size 0), matching the Dockerfile's
// `[ ! -s "$secret" ]` guard, which fails the build in both cases. This
// surfaces the cause early. Only a warning — cocoon does not create the file
// (it is the user's secret) and does not read its contents; a non-empty file
// with no usable SUDO_PASSWORD= line is left for the build to reject. Stat
// errors other than ErrNotExist are ignored: a best-effort nudge, not a gate.
func warnPasswordSudoMissingSecret(
	ctx *generate.WorkspaceContext, outDir string, log *logx.Logger, cat *i18n.Catalog,
) {
	if !ctx.PasswordSudoEnabled() {
		return
	}
	secret := filepath.Join(outDir, ".devcontainer", generate.SudoPasswordSecretFile)
	info, err := os.Stat(secret)
	missing := errors.Is(err, os.ErrNotExist)
	empty := err == nil && info.Size() == 0
	if missing || empty {
		log.Warn(cat.Msg("gen_password_sudo_missing_secret", secret))
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
