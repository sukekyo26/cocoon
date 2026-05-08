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

// errInputRequired is returned by huh input validators when the user submits
// an empty value.
var errInputRequired = errors.New("required")

// errURLInDescription is returned by the description-input validator when the
// user submits a description that does not embed an upstream URL.
var errURLInDescription = errors.New("include URL in parentheses, e.g. \"(https://...)\"")

// errPathNotADir is returned by dirExists when the candidate path exists but
// points at a non-directory entry.
var errPathNotADir = errors.New("not a directory")

// scaffoldOpts collects all values that drive code generation. The CLI flag
// parser fills in whatever the user supplied; remaining holes are filled by
// the interactive form (unless --non-interactive was passed).
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

	// Track which flags were explicitly set so the interactive form can
	// distinguish "user passed --default=false" from "user did not pass --default".
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

// applyPickedTemplate transfers a non-empty template selection from the form
// closure into opts. Extracted from the inline defer so the assignment is
// reachable from unit tests without invoking the huh form.
func applyPickedTemplate(opts *scaffoldOpts, picked string) {
	if picked != "" {
		opts.template = templateKind(picked)
	}
}

// prompter abstracts the interactive form runner so tests can substitute a
// deterministic fake for the huh.Form. The interface is unexported because
// the only production implementation is huhPrompter and callers outside the
// package have no business swapping it.
type prompter interface {
	Run(groups []*huh.Group) error
}

// huhPrompter is the production prompter backed by charmbracelet/huh.
type huhPrompter struct{}

// Run implements [prompter] by assembling a huh.Form from the supplied groups
// and invoking form.Run. Empty groups short-circuit to nil so callers can
// safely pass an unconditional slice.
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
				Value(&opts.withInstallUser),
		))
	}

	if err := p.Run(groups); err != nil {
		return err //nolint:wrapcheck // prompter implementations are responsible for their own wrapping
	}
	applyPickedTemplate(opts, pickedTemplate)
	return nil
}

// finalizeOpts performs the cross-field validation that applies to both
// interactive and non-interactive modes (e.g. tarball implies version_capable).
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
		// Truly-missing flags (empty string / not provided) keep the existing
		// `plugin_scaffold_missing_flag` actionable message that names the
		// missing flag explicitly. Only non-empty inputs reach the shared
		// validators, whose sentinel errors are mapped to localized keys so
		// raw English from err.Error() never leaks into ja output.
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
				// Defensive fallback for future sentinels not yet in this
				// switch; emit a localized generic message so non-ja text
				// never reaches output.
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

func runScaffold(opts scaffoldOpts, cat *i18n.Catalog, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
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

	written := make([]string, 0, 3)
	cleanup := func() {
		if dirExisted {
			// We did not create the directory, so leave it alone — the
			// caller used --force and is in charge.
			return
		}
		_ = os.RemoveAll(dir)
	}

	tomlBody, err := renderPluginTOML(data)
	if err != nil {
		cleanup()
		log.Errorf("ERROR: render plugin.toml: %s", err)
		return ErrFailure
	}
	tomlPath := filepath.Join(dir, "plugin.toml")
	if writeErr := fsx.AtomicWriteFile(tomlPath, []byte(tomlBody), 0o644); writeErr != nil {
		cleanup()
		log.Errorf("ERROR: write %s: %s", tomlPath, writeErr)
		return ErrFailure
	}
	written = append(written, tomlPath)

	installBody, err := renderInstallSh(data)
	if err != nil {
		cleanup()
		log.Errorf("ERROR: render install.sh: %s", err)
		return ErrFailure
	}
	installPath := filepath.Join(dir, "install.sh")
	if writeErr := fsx.AtomicWriteFile(installPath, []byte(installBody), 0o755); writeErr != nil {
		cleanup()
		log.Errorf("ERROR: write %s: %s", installPath, writeErr)
		return ErrFailure
	}
	written = append(written, installPath)

	if opts.withInstallUser {
		userBody, renderErr := renderInstallUserSh(data)
		if renderErr != nil {
			cleanup()
			log.Errorf("ERROR: render install_user.sh: %s", renderErr)
			return ErrFailure
		}
		userPath := filepath.Join(dir, "install_user.sh")
		if writeErr := fsx.AtomicWriteFile(userPath, []byte(userBody), 0o755); writeErr != nil {
			cleanup()
			log.Errorf("ERROR: write %s: %s", userPath, writeErr)
			return ErrFailure
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
