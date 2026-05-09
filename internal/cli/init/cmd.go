package initcli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/setup"
)

const initLong = `cocoon init — generate workspace.toml in the current directory

Asks (when running interactively) which mount range to use, whether to
emit .devcontainer/devcontainer.json, and which categories of common apt
packages to install. The answers are written into a fresh workspace.toml
at the project root.

Use --yes plus the --mount-root / --devcontainer / --apt-categories
flags to drive non-interactively from CI.`

// NewCommand returns the cobra command for ` + "`cocoon init`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	flags := initFlags{
		AutoYes:        false,
		MountRoot:      "",
		Devcontainer:   false,
		NoDevcontainer: false,
		AptCategories:  "",
		Force:          false,
	}
	cmd := &cobra.Command{
		Use:           "init",
		Short:         "Create workspace.toml in the current directory",
		Long:          initLong,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd, stdout, stderr, &flags)
		},
	}
	cmd.Flags().BoolVar(&flags.AutoYes, "yes", false, "skip interactive prompts and use defaults")
	cmd.Flags().StringVar(&flags.MountRoot, "mount-root", "", `mount range: "." (cwd, default) or ".." (parent)`)
	cmd.Flags().BoolVar(&flags.Devcontainer, "devcontainer", false, "force-enable .devcontainer/devcontainer.json output")
	cmd.Flags().BoolVar(&flags.NoDevcontainer, "no-devcontainer", false, "skip .devcontainer/devcontainer.json output")
	cmd.Flags().StringVar(
		&flags.AptCategories,
		"apt-categories",
		"",
		"comma-separated apt category IDs (skips the multi-select prompt)",
	)
	cmd.Flags().BoolVar(&flags.Force, "force", false, "overwrite an existing workspace.toml")
	return cmd
}

type initFlags struct {
	AutoYes        bool
	MountRoot      string
	Devcontainer   bool
	NoDevcontainer bool
	AptCategories  string
	Force          bool
}

func runInit(cmd *cobra.Command, stdout, _ io.Writer, flags *initFlags) error {
	if flags.Devcontainer && flags.NoDevcontainer {
		return fmt.Errorf("%w: --devcontainer and --no-devcontainer are mutually exclusive", ErrUsage)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailure, err)
	}
	target := filepath.Join(cwd, "workspace.toml")
	if _, statErr := os.Stat(target); statErr == nil && !flags.Force {
		return fmt.Errorf("%w: %s already exists; use --force to overwrite", ErrUsage, target)
	}

	mountRoot, err := resolveMountRoot(flags)
	if err != nil {
		return err
	}
	devcontainer, err := resolveDevcontainer(flags)
	if err != nil {
		return err
	}
	aptCatIDs, err := resolveAptCategories(flags)
	if err != nil {
		return err
	}

	pkgs := setup.ExpandAptCategories(aptCatIDs)
	content := renderWorkspaceToml(filepath.Base(cwd), mountRoot, devcontainer, pkgs)
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil { //nolint:gosec // workspace.toml is user-readable.
		return fmt.Errorf("%w: write %s: %w", ErrFailure, target, err)
	}

	fmt.Fprintf(stdout, "wrote %s\n", target)
	printNextSteps(stdout, devcontainer)
	_ = cmd // reserved for future ctx-aware flows
	return nil
}

func printNextSteps(stdout io.Writer, devcontainer bool) {
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Next steps:")
	fmt.Fprintln(stdout, "  1. cocoon gen")
	fmt.Fprintln(stdout, "  2. docker compose -f .devcontainer/docker-compose.yml up -d")
	if devcontainer {
		fmt.Fprintln(stdout, `     (or open in VS Code → "Reopen in Container")`)
	}
}

func resolveMountRoot(flags *initFlags) (string, error) {
	if flags.MountRoot != "" {
		if flags.MountRoot != "." && flags.MountRoot != ".." {
			return "", fmt.Errorf(`%w: --mount-root must be "." or ".."`, ErrUsage)
		}
		return flags.MountRoot, nil
	}
	if flags.AutoYes {
		return ".", nil
	}
	v := "."
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Mount range").
				Description("How much of your filesystem should be visible inside the container?").
				Options(
					huh.NewOption("Just this project (.)", "."),
					huh.NewOption("Parent directory — sibling repos visible (..)", ".."),
				).
				Value(&v),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("%w: mount-root prompt: %w", ErrFailure, err)
	}
	return v, nil
}

func resolveDevcontainer(flags *initFlags) (bool, error) {
	if flags.Devcontainer {
		return true, nil
	}
	if flags.NoDevcontainer {
		return false, nil
	}
	if flags.AutoYes {
		return true, nil
	}
	v := true
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Generate .devcontainer/devcontainer.json for VS Code Dev Containers?").
				Description("Says yes if you ever open this repo in VS Code Dev Containers; harmless otherwise.").
				Affirmative("Yes").
				Negative("No").
				Value(&v),
		),
	)
	if err := form.Run(); err != nil {
		return false, fmt.Errorf("%w: devcontainer prompt: %w", ErrFailure, err)
	}
	return v, nil
}

func resolveAptCategories(flags *initFlags) ([]string, error) {
	if flags.AptCategories != "" {
		var ids []string
		for _, raw := range strings.Split(flags.AptCategories, ",") {
			id := strings.TrimSpace(raw)
			if id == "" {
				continue
			}
			if setup.AptCategoryByID(id) == nil {
				return nil, fmt.Errorf("%w: unknown apt category %q (run `cocoon init --help` for the list)", ErrUsage, id)
			}
			ids = append(ids, id)
		}
		return ids, nil
	}
	if flags.AutoYes {
		return setup.DefaultAptCategoryIDs(), nil
	}
	selected := setup.DefaultAptCategoryIDs()
	options := make([]huh.Option[string], len(setup.AptCategories))
	for i, c := range setup.AptCategories {
		options[i] = huh.NewOption(fmt.Sprintf("%s (%s)", c.Label, c.Description), c.ID)
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select common apt packages to install").
				Description("Pre-checked categories are installed by default; uncheck what you do not need.").
				Options(options...).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("%w: apt-categories prompt: %w", ErrFailure, err)
	}
	return selected, nil
}

func renderWorkspaceToml(serviceName, mountRoot string, devcontainer bool, pkgs []string) string {
	var sb strings.Builder
	sb.WriteString("# workspace.toml — cocoon configuration (generated by `cocoon init`)\n")
	sb.WriteString("# Edit freely; re-run `cocoon gen` to regenerate .devcontainer/.\n\n")

	sb.WriteString("[workspace]\n")
	fmt.Fprintf(&sb, "mount_root = %q\n", mountRoot)
	fmt.Fprintf(&sb, "devcontainer = %t\n\n", devcontainer)

	sb.WriteString("[container]\n")
	fmt.Fprintf(&sb, "service_name = %q\n", serviceName)
	sb.WriteString(`username = "shogo"` + "\n")
	sb.WriteString(`os = "ubuntu"` + "\n")
	sb.WriteString(`os_version = "24.04"` + "\n\n")

	sb.WriteString("[plugins]\n")
	sb.WriteString("enable = []\n\n")

	sb.WriteString("[apt]\n")
	if len(pkgs) == 0 {
		sb.WriteString("packages = []\n")
		return sb.String()
	}
	sb.WriteString("packages = [\n")
	for _, pkg := range pkgs {
		fmt.Fprintf(&sb, "  %q,\n", pkg)
	}
	sb.WriteString("]\n")
	return sb.String()
}
