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
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/fsx"
	"github.com/sukekyo26/cocoon/internal/generate/codeworkspace"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/logx"
)

func newWorkspaceCmd(stdout, stderr io.Writer) *cobra.Command {
	cat := i18n.New(i18n.Detect())
	var (
		workspaceFlag string
		outputFlag    string
		nameFlag      string
		folderFlags   []string
	)
	cmd := &cobra.Command{
		Use:           "workspace",
		Short:         cat.Msg("cmd_gen_workspace_short"),
		Long:          cat.Msg("cmd_gen_workspace_long"),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runGenWorkspace(stdout, stderr, workspaceFlag, outputFlag, nameFlag, folderFlags)
		},
	}
	cmd.Flags().StringVar(&workspaceFlag, "workspace", "", cat.Msg("flag_gen_workspace_workspace_usage"))
	cmd.Flags().StringVar(&outputFlag, "output", "", cat.Msg("flag_gen_workspace_output_usage"))
	cmd.Flags().StringVar(&nameFlag, "name", "", cat.Msg("flag_gen_workspace_name_usage"))
	cmd.Flags().StringArrayVar(&folderFlags, "folder", nil, cat.Msg("flag_gen_workspace_folder_usage"))
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", clihelpers.ErrUsage, err)
	})
	clihelpers.AttachHelpAlias(cmd)
	return cmd
}

func runGenWorkspace(
	stdout, stderr io.Writer,
	workspaceFlag, outputFlag, nameFlag string,
	folderFlags []string,
) error {
	cat := i18n.New(i18n.Detect())
	log := logx.New(stdout, stderr)
	outDir, ctx, err := loadGenContext(stderr, workspaceFlag, outputFlag)
	if err != nil {
		return err
	}
	name, err := resolveWorkspaceName(nameFlag, ctx.WS.CodeWorkspace, ctx.ProjectDir, cat)
	if err != nil {
		return err
	}
	extras, err := parseFolderFlags(folderFlags)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("%w: resolve home dir: %w", clihelpers.ErrFailure, err)
	}
	target := filepath.Join(outDir, name+".code-workspace")
	// The .code-workspace file resolves relative paths from its own
	// directory, so anchor relativization on filepath.Dir(target) — which
	// matches outDir but stays explicit if a future tweak nests deeper.
	body, err := codeworkspace.Generate(ctx, codeworkspace.Options{
		ExtraFolders: extras,
		HomeDir:      home,
		OutputDir:    filepath.Dir(target),
	})
	if err != nil {
		return mapWorkspaceErr(err, cat)
	}
	if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); mkErr != nil {
		return fmt.Errorf("%w: mkdir %s: %w", clihelpers.ErrFailure, target, mkErr)
	}
	if wErr := fsx.AtomicWriteFile(target, []byte(body), 0o644); wErr != nil {
		return fmt.Errorf("%w: write %s: %w", clihelpers.ErrFailure, target, wErr)
	}

	cwd, _ := os.Getwd() //nolint:errcheck // cwd is best-effort for pretty-printing only.
	log.Success(cat.Msg("gen_workspace_wrote", displayPath(cwd, target)))
	log.Info("")
	log.Info(log.Bold(cat.Msg("gen_workspace_next_header")))
	log.Info(cat.Msg("gen_workspace_next_step", displayPath(cwd, target)))
	return nil
}

// resolveWorkspaceName follows the precedence: --name > [code_workspace].name
// > filepath.Base(projectDir). Explicit values (--name or spec.Name) must
// pass the same validator as the workspace.toml field — passing "." or
// "/foo" would otherwise produce a broken or escape-the-root output path.
// Derived values (basename of projectDir) are *sanitized* rather than
// validated, since a basename like "my-project" is benign but a quirky
// tempdir name should not block generation.
func resolveWorkspaceName(
	flag string,
	spec *config.CodeWorkspaceSpec,
	projectDir string,
	cat *i18n.Catalog,
) (string, error) {
	if flag != "" {
		if !config.IsValidCodeWorkspaceName(flag) {
			return "", fmt.Errorf("%w: %s", clihelpers.ErrUsage, cat.Msg("gen_workspace_invalid_name", flag))
		}
		return flag, nil
	}
	if spec != nil && spec.Name != "" {
		// spec.Name was already validated by config.Validate at load time.
		return spec.Name, nil
	}
	name := filepath.Base(projectDir)
	if name == "" || name == "." || name == "/" || !config.IsValidCodeWorkspaceName(name) {
		return "workspace", nil
	}
	return name, nil
}

// parseFolderFlags converts repeated --folder values into CodeWorkspaceFolder
// records. Each value may be "<path>" (name auto-derived later) or
// "<path>=<name>" (explicit override). Empty values are an ErrUsage.
func parseFolderFlags(flags []string) ([]config.CodeWorkspaceFolder, error) {
	if len(flags) == 0 {
		return nil, nil
	}
	out := make([]config.CodeWorkspaceFolder, 0, len(flags))
	for _, raw := range flags {
		if raw == "" {
			return nil, fmt.Errorf("%w: --folder value must not be empty", clihelpers.ErrUsage)
		}
		path, name, _ := strings.Cut(raw, "=")
		if path == "" {
			return nil, fmt.Errorf("%w: --folder %q has empty path before \"=\"", clihelpers.ErrUsage, raw)
		}
		out = append(out, config.CodeWorkspaceFolder{Path: path, Name: name})
	}
	return out, nil
}

// mapWorkspaceErr translates codeworkspace.Err* sentinels into CLI errors
// with usage vs failure classification and localized messages where
// helpful. Unmatched errors fall through as ErrFailure-wrapped.
func mapWorkspaceErr(err error, cat *i18n.Catalog) error {
	switch {
	case errors.Is(err, codeworkspace.ErrNoFolders):
		return fmt.Errorf("%w: %s", clihelpers.ErrUsage, cat.Msg("gen_workspace_no_folders"))
	case errors.Is(err, codeworkspace.ErrInvalidFolderPath):
		return fmt.Errorf("%w: %w", clihelpers.ErrUsage, err)
	case errors.Is(err, codeworkspace.ErrMissingHomeDir):
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	default:
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
}
