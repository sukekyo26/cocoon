package plugincli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/fsx"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

var (
	errInputRequired    = errors.New("required")
	errURLInDescription = errors.New("include URL in parentheses, e.g. \"(https://...)\"")
	errPathNotADir      = errors.New("not a directory")
)

// scaffoldOpts collects all values that drive code generation; holes the
// user did not pass via flags are filled by the interactive form (unless
// --non-interactive).
type scaffoldOpts struct {
	id              string
	pluginsDir      string
	name            string
	description     string
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
	setDefaultEnabled  bool
	setRequiresRoot    bool
	setVersionCapable  bool
	setTemplate        bool
	setWithInstallUser bool
}

func validateID(id string, cat *i18n.Catalog, stderr io.Writer) error {
	log := logx.New(io.Discard, stderr)
	if id == "" {
		log.Error("ERROR: " + cat.Msg("plugin_scaffold_missing_id"))
		return ErrUsage
	}
	if !config.IsValidPluginID(id) {
		log.Error("ERROR: " + cat.Msg("plugin_scaffold_invalid_id", id))
		return ErrUsage
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

// validateDescriptionInput requires both a non-empty value and an embedded URL.
func validateDescriptionInput(s string) error {
	if strings.TrimSpace(s) == "" {
		return errInputRequired
	}
	if !strings.Contains(s, "(http") {
		return errURLInDescription
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
			return ErrCanceled
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
					huh.NewOption("generic — apt / .deb / freeform", string(tmplGeneric)),
					huh.NewOption("curl-pipe — `curl ... | bash` (uv, proto)", string(tmplCurlPipe)),
					huh.NewOption("tarball — GitHub Release tarball + sha256 (starship, go)", string(tmplTarball)),
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
// non-interactive modes (e.g. tarball implies version_capable).
func finalizeOpts(opts *scaffoldOpts, cat *i18n.Catalog, stderr io.Writer) error {
	log := logx.New(io.Discard, stderr)
	switch opts.template {
	case tmplCurlPipe, tmplTarball, tmplGeneric:
		// ok
	default:
		log.Error("ERROR: " + cat.Msg("plugin_scaffold_unknown_template", string(opts.template)))
		return ErrUsage
	}

	if opts.nonInteractive {
		// Only non-empty inputs reach the shared validators so the localized
		// `plugin_scaffold_missing_flag` actionable message wins over the
		// English sentinel for the "missing" case.
		if opts.name == "" {
			log.Error("ERROR: " + cat.Msg("plugin_scaffold_missing_flag", "name"))
			return ErrUsage
		}
		if opts.description == "" {
			log.Error("ERROR: " + cat.Msg("plugin_scaffold_missing_flag", "description"))
			return ErrUsage
		}
		if err := validateNameInput(opts.name); err != nil {
			log.Error("ERROR: " + cat.Msg("plugin_scaffold_blank_name"))
			return ErrUsage
		}
		if err := validateDescriptionInput(opts.description); err != nil {
			switch {
			case errors.Is(err, errInputRequired):
				log.Error("ERROR: " + cat.Msg("plugin_scaffold_blank_description"))
			case errors.Is(err, errURLInDescription):
				log.Error("ERROR: " + cat.Msg("plugin_scaffold_desc_missing_url"))
			default:
				log.Error("ERROR: " + cat.Msg("plugin_scaffold_desc_invalid"))
			}
			return ErrUsage
		}
	}

	if opts.template == tmplTarball && !opts.versionCapable {
		log.Error("ERROR: " + cat.Msg("plugin_scaffold_tarball_needs_ver"))
		return ErrUsage
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
	log *logx.Logger, cleanup func(),
) (string, error) {
	body, err := render()
	if err != nil {
		cleanup()
		log.Errorf("ERROR: render %s: %s", name, err)
		return "", ErrFailure
	}
	path := filepath.Join(dir, name)
	if writeErr := fsx.AtomicWriteFile(path, []byte(body), mode); writeErr != nil {
		cleanup()
		log.Errorf("ERROR: write %s: %s", path, writeErr)
		return "", ErrFailure
	}
	return path, nil
}

func runScaffold(opts scaffoldOpts, cat *i18n.Catalog, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	pluginsDir, err := resolvePluginsDir(opts)
	switch {
	case errors.Is(err, ErrWorkspaceNotFound):
		log.Error("ERROR: " + cat.Msg("plugin_scaffold_no_plugins_dir"))
		return ErrUsage
	case err != nil:
		log.Errorf("ERROR: resolve plugins dir: %s", err)
		return ErrFailure
	}
	opts.pluginsDir = pluginsDir
	dir := filepath.Join(opts.pluginsDir, opts.id)
	dirExisted, err := dirExists(dir)
	if err != nil {
		log.Errorf("ERROR: %s: %s", dir, err)
		return ErrFailure
	}
	if dirExisted && !opts.force {
		log.Error("ERROR: " + cat.Msg("plugin_scaffold_dir_exists", dir))
		return ErrFailure
	}
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		log.Errorf("ERROR: mkdir %s: %s", dir, mkErr)
		return ErrFailure
	}

	data := scaffoldData{
		ID:             opts.id,
		Name:           opts.name,
		Description:    opts.description,
		Default:        opts.defaultEnabled,
		RequiresRoot:   opts.requiresRoot,
		VersionCapable: opts.versionCapable,
		Template:       opts.template,
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
		func() (string, error) { return renderPluginTOML(data) }, log, cleanup)
	if terr != nil {
		return terr
	}
	written = append(written, tomlPath)

	installPath, ierr := renderAndWrite(dir, "install.sh", 0o755,
		func() (string, error) { return renderInstallSh(data) }, log, cleanup)
	if ierr != nil {
		return ierr
	}
	written = append(written, installPath)

	if opts.withInstallUser {
		userPath, uerr := renderAndWrite(dir, "install_user.sh", 0o755,
			func() (string, error) { return renderInstallUserSh(data) }, log, cleanup)
		if uerr != nil {
			return uerr
		}
		written = append(written, userPath)
	}

	// Re-load the freshly written plugin.toml to confirm strict validation.
	if _, err := plugin.Load(tomlPath); err != nil {
		cleanup()
		log.Error("ERROR: " + cat.Msg("plugin_scaffold_validation_failed"))
		log.Errorf("  %s", err)
		return ErrFailure
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
