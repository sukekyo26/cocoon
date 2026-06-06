package plugincli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/fsx"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

var (
	errInputRequired = errors.New("required")
	errInvalidURL    = errors.New("must start with https:// and contain no whitespace")
	errPathNotADir   = errors.New("not a directory")
)

// scaffoldOpts collects all values that drive code generation; holes the
// user did not pass via flags are filled by the interactive form (unless
// --non-interactive).
type scaffoldOpts struct {
	id              string
	pluginsDir      string
	name            string
	description     string
	url             string
	defaultEnabled  bool
	requiresRoot    bool
	versionCapable  bool
	template        templateKind
	withInstallUser bool
	nonInteractive  bool
	force           bool

	// set* lets the form distinguish "user passed --default=false" from
	// "user did not pass --default".
	setName            bool
	setDescription     bool
	setURL             bool
	setDefaultEnabled  bool
	setRequiresRoot    bool
	setVersionCapable  bool
	setTemplate        bool
	setWithInstallUser bool
}

func validateID(id string) error {
	if id == "" {
		return clihelpers.UsageErr("plugin_scaffold_missing_id")
	}
	if !config.IsValidPluginID(id) {
		return clihelpers.UsageErr("plugin_scaffold_invalid_id", id)
	}
	return nil
}

// validateNameInput rejects empty/whitespace-only display names.
func validateNameInput(s string) error {
	if strings.TrimSpace(s) == "" {
		return errInputRequired
	}
	return nil
}

// validateDescriptionInput rejects empty/whitespace-only descriptions.
// The upstream URL travels in a separate `url` field now.
func validateDescriptionInput(s string) error {
	if strings.TrimSpace(s) == "" {
		return errInputRequired
	}
	return nil
}

// validateURLInput rejects empty values and anything that is not a plain
// https:// URL. Whitespace anywhere in the value is rejected so it can be
// embedded verbatim in plugin.toml.
func validateURLInput(s string) error {
	if strings.TrimSpace(s) == "" {
		return errInputRequired
	}
	if !strings.HasPrefix(s, "https://") || strings.ContainsAny(s, " \t\r\n") {
		return errInvalidURL
	}
	return nil
}

// applyPickedTemplate transfers a non-empty template selection into opts.
func applyPickedTemplate(opts *scaffoldOpts, picked string) {
	if picked != "" {
		opts.template = templateKind(picked)
	}
}

// prompter abstracts the interactive form so tests can substitute a fake.
type prompter interface {
	Run(groups []*huh.Group) error
}

// huhPrompter is the charmbracelet/huh backed prompter.
type huhPrompter struct{}

// Run short-circuits to nil on empty groups so callers can pass an
// unconditional slice.
func (huhPrompter) Run(groups []*huh.Group) error {
	if len(groups) == 0 {
		return nil
	}
	form := huh.NewForm(groups...)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return clihelpers.ErrCanceled
		}
		return fmt.Errorf("plugin scaffold: prompt: %w", err)
	}
	return nil
}

// promptMissing fills in any field the user omitted via flags by running an
// interactive huh form. Fields the user already passed via flags are skipped.
//
//nolint:gocognit // sequential per-field prompt assembly; splitting hides intent.
func promptMissing(opts *scaffoldOpts, cat *i18n.Catalog, p prompter) error {
	var pickedTemplate string
	groups := make([]*huh.Group, 0, 8)

	if !opts.setName {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title(cat.Msg("plugin_scaffold_prompt_name")).
				Value(&opts.name).
				Validate(validateNameInput),
		))
	}
	if !opts.setDescription {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title(cat.Msg("plugin_scaffold_prompt_desc")).
				Value(&opts.description).
				Validate(validateDescriptionInput),
		))
	}
	if !opts.setURL {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title(cat.Msg("plugin_scaffold_prompt_url")).
				Value(&opts.url).
				Validate(validateURLInput),
		))
	}
	if !opts.setDefaultEnabled {
		groups = append(groups, huh.NewGroup(
			huh.NewConfirm().
				Title(cat.Msg("plugin_scaffold_prompt_default")).
				Value(&opts.defaultEnabled),
		))
	}
	if !opts.setRequiresRoot {
		groups = append(groups, huh.NewGroup(
			huh.NewConfirm().
				Title(cat.Msg("plugin_scaffold_prompt_root")).
				Value(&opts.requiresRoot),
		))
	}
	if !opts.setVersionCapable {
		groups = append(groups, huh.NewGroup(
			huh.NewConfirm().
				Title(cat.Msg("plugin_scaffold_prompt_versioned")).
				Value(&opts.versionCapable),
		))
	}
	if !opts.setTemplate {
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title(cat.Msg("plugin_scaffold_prompt_template")).
				Options(
					huh.NewOption("installer — vendor curl|bash installer (bun, uv, rust)", string(tmplInstaller)),
					huh.NewOption("binary    — single binary from GitHub Release (kubectl, helm)", string(tmplBinary)),
					huh.NewOption("apt       — apt repo or .deb install (docker-cli, google-chrome)", string(tmplApt)),
					huh.NewOption("archive   — multi-file tar/zip extract (go, node, zig)", string(tmplArchive)),
				).
				Value(&pickedTemplate),
		))
	}
	if !opts.setWithInstallUser {
		groups = append(groups, huh.NewGroup(
			huh.NewConfirm().
				Title(cat.Msg("plugin_scaffold_prompt_user_hook")).
				Description(cat.Msg("plugin_scaffold_prompt_user_hook_desc")).
				Value(&opts.withInstallUser),
		))
	}

	if err := p.Run(groups); err != nil {
		return err //nolint:wrapcheck // prompter implementations are responsible for their own wrapping
	}
	applyPickedTemplate(opts, pickedTemplate)
	return nil
}

// finalizeOpts runs the cross-field rules shared by both interactive and
// non-interactive modes (e.g. binary implies version_capable).
func finalizeOpts(opts *scaffoldOpts) error {
	switch opts.template {
	case tmplInstaller, tmplBinary, tmplApt, tmplArchive:
		// ok
	default:
		return clihelpers.UsageErr("plugin_scaffold_unknown_template", string(opts.template))
	}

	if opts.nonInteractive {
		// Only non-empty inputs reach the shared validators so the localized
		// `plugin_scaffold_missing_flag` actionable message wins over the
		// English sentinel for the "missing" case.
		if opts.name == "" {
			return clihelpers.UsageErr("plugin_scaffold_missing_flag", "name")
		}
		if opts.description == "" {
			return clihelpers.UsageErr("plugin_scaffold_missing_flag", "description")
		}
		if opts.url == "" {
			return clihelpers.UsageErr("plugin_scaffold_missing_flag", "url")
		}
		if err := validateNameInput(opts.name); err != nil {
			return clihelpers.UsageErr("plugin_scaffold_blank_name")
		}
		if err := validateDescriptionInput(opts.description); err != nil {
			return clihelpers.UsageErr("plugin_scaffold_blank_description")
		}
		if err := validateURLInput(opts.url); err != nil {
			if errors.Is(err, errInputRequired) {
				return clihelpers.UsageErr("plugin_scaffold_blank_url")
			}
			return clihelpers.UsageErr("plugin_scaffold_invalid_url")
		}
	}

	if opts.template == tmplBinary && !opts.versionCapable {
		return clihelpers.UsageErr("plugin_scaffold_binary_needs_ver")
	}
	return nil
}

// resolvePluginsDir auto-discovers <workspace>/.cocoon/plugins when
// --plugins-dir is unset. Error semantics mirror projectPluginsDir.
func resolvePluginsDir(opts scaffoldOpts) (string, error) {
	if opts.pluginsDir != "" {
		return opts.pluginsDir, nil
	}
	return projectPluginsDir()
}

// renderAndWrite triggers cleanup() on any failure. Returns the path
// written (relative iff dir was relative).
func renderAndWrite(
	dir, name string, mode os.FileMode,
	render func() (string, error),
	cleanup func(),
) (string, error) {
	body, err := render()
	if err != nil {
		cleanup()
		return "", clihelpers.FailureWrap(err, "")
	}
	path := filepath.Join(dir, name)
	if writeErr := fsx.AtomicWriteFile(path, []byte(body), mode); writeErr != nil {
		cleanup()
		return "", clihelpers.FailureWrap(writeErr, "")
	}
	return path, nil
}

func runScaffold(opts scaffoldOpts, cat *i18n.Catalog, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	pluginsDir, err := resolvePluginsDir(opts)
	switch {
	case errors.Is(err, ErrWorkspaceNotFound):
		return clihelpers.UsageErr("plugin_scaffold_no_plugins_dir")
	case err != nil:
		return clihelpers.FailureWrap(err, "")
	}
	opts.pluginsDir = pluginsDir
	dir := filepath.Join(opts.pluginsDir, opts.id)
	dirExisted, err := dirExists(dir)
	if err != nil {
		return clihelpers.FailureWrap(err, "")
	}
	if dirExisted && !opts.force {
		return clihelpers.FailureErr("plugin_scaffold_dir_exists", dir)
	}
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return clihelpers.FailureWrap(mkErr, "")
	}

	data := scaffoldData{
		ID:             opts.id,
		Name:           opts.name,
		Description:    opts.description,
		URL:            opts.url,
		Default:        opts.defaultEnabled,
		RequiresRoot:   opts.requiresRoot,
		VersionCapable: opts.versionCapable,
		Template:       opts.template,
		MethodDesc:     methodDescription(opts.template),
		WithUserHook:   opts.withInstallUser,
	} //nolint:exhaustruct // every field is populated above

	cleanup := func() {
		if dirExisted {
			// We did not create the directory, so leave it alone — the
			// caller used --force and is in charge.
			return
		}
		_ = os.RemoveAll(dir)
	}

	written := make([]string, 0, 3)
	tomlPath, terr := renderAndWrite(dir, "plugin.toml", 0o644,
		func() (string, error) { return renderPluginTOML(data) }, cleanup)
	if terr != nil {
		return terr
	}
	written = append(written, tomlPath)

	installPath, ierr := renderAndWrite(dir, installScriptName(opts.template), 0o755,
		func() (string, error) { return renderInstallSh(data) }, cleanup)
	if ierr != nil {
		return ierr
	}
	written = append(written, installPath)

	if opts.withInstallUser {
		userPath, uerr := renderAndWrite(dir, "install_user.sh", 0o755,
			func() (string, error) { return renderInstallUserSh(data) }, cleanup)
		if uerr != nil {
			return uerr
		}
		written = append(written, userPath)
	}

	// Re-load the freshly written plugin.toml to confirm strict validation.
	if _, err := plugin.Load(tomlPath); err != nil {
		cleanup()
		return clihelpers.FailureWrap(err, "plugin_scaffold_validation_failed")
	}

	log.Info(cat.Msg("plugin_scaffold_done", dir, len(written)))
	return nil
}

func dirExists(path string) (bool, error) {
	st, err := os.Stat(path)
	if err == nil {
		if !st.IsDir() {
			return false, fmt.Errorf("%s: %w", path, errPathNotADir)
		}
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("stat %s: %w", path, err)
}
