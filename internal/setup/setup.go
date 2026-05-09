// Package setup implements the interactive workspace bootstrap flow that used to live in setup-docker.sh.
package setup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sukekyo26/cocoon/internal/certificates"
	generatecli "github.com/sukekyo26/cocoon/internal/cli/generate"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/doctor"
	"github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/hostguard"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/repositories"
	"github.com/sukekyo26/cocoon/internal/tui"
)

// Sentinel errors propagated to the binary boundary for exit-code mapping.
var (
	ErrConfig          = errors.New("setup: invalid options")
	ErrPrereq          = errors.New("prerequisite check failed")
	ErrCanceled        = errors.New("canceled")
	ErrDiffFound       = errors.New("changes detected")
	ErrInsideContainer = errors.New("setup cannot run from inside a container with [home_files] configured")
	ErrInvalidInput    = errors.New("invalid input")
	ErrHomeFiles       = errors.New("home_files")
	ErrTemplate        = errors.New("template")
	ErrDockerGID       = errors.New("docker gid")
)

// Translator is the i18n catalog subset used by setup.
type Translator interface {
	Msg(key string, args ...any) string
}

// Generator produces workspace artifact files.
type Generator interface {
	GenerateAll(wsPath, pluginsDir, outputDir string, stderr io.Writer) error
}

// RepoCloner clones companion repositories.
type RepoCloner interface {
	CloneAll(scriptDir string, log func(level, msg string)) (repositories.CloneSummary, error)
}

// DockerGIDDetector detects the Docker socket GID.
type DockerGIDDetector interface {
	Detect() (int, error)
}

// Options configures Run.
type Options struct {
	WorkspaceDir string
	ForceInit    bool
	AutoYes      bool
	RunDoctor    bool
	RunDiff      bool
	NoClone      bool
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	Logger       *logx.Logger
	Catalog      Translator
	Selector     tui.Selector
	Generator    Generator
	Cloner       RepoCloner
	GIDDetector  DockerGIDDetector
	PluginsDir   string
}

// Run drives the full setup flow.
//
//nolint:gocognit,gocyclo,funlen // top-level setup pipeline; readers expect every step inline.
func Run(opts Options) error {
	defaults(&opts)
	if opts.Catalog == nil {
		return fmt.Errorf("%w: Catalog is required", ErrConfig)
	}
	if opts.WorkspaceDir == "" {
		return fmt.Errorf("%w: WorkspaceDir is required", ErrConfig)
	}
	t := opts.Catalog
	log := opts.Logger

	if opts.RunDoctor {
		if doctor.Run(doctor.Options{Root: opts.WorkspaceDir}, opts.Stdout) {
			return nil
		}
		return fmt.Errorf("%w: doctor reported failures", ErrPrereq)
	}

	pluginsDir := opts.PluginsDir
	if pluginsDir == "" {
		pluginsDir = filepath.Join(opts.WorkspaceDir, "plugins")
	}
	wsPath := filepath.Join(opts.WorkspaceDir, "workspace.toml")

	hasContainer := hasContainerSection(wsPath)

	var ws *config.Workspace
	if hasContainer && !opts.ForceInit {
		log.Info(sectionHeader(t.Msg("setup_header_regenerate")))
		loaded, err := config.LoadWorkspace(wsPath)
		if err != nil {
			return fmt.Errorf("load workspace.toml: %w", err)
		}
		ws = loaded
		log.Info(t.Msg("setup_service_info", ws.Container.ServiceName))
		log.Info(t.Msg("setup_username_info", ws.Container.Username))
		log.Info(t.Msg("setup_plugins_info", strings.Join(ws.Plugins.Enable, ", ")))
	} else {
		log.Info(sectionHeader(t.Msg("setup_header_generate")))
		w, err := runInteractive(opts, wsPath, pluginsDir)
		if err != nil {
			return err
		}
		ws = w
	}

	if err := checkHomeFilesHostOnly(ws, log, t, hostguard.InsideContainer); err != nil {
		return err
	}

	uid := os.Getuid()
	gid := os.Getgid()

	dockerGID, err := opts.GIDDetector.Detect()
	if err != nil {
		log.Error(t.Msg("setup_docker_gid_failed"))
		log.Error(t.Msg("setup_docker_gid_hint"))
		return fmt.Errorf("%w: docker gid: %w", ErrPrereq, err)
	}
	log.Info(t.Msg("setup_detected_docker_gid", strconv.Itoa(dockerGID)))

	if opts.RunDiff {
		return runDiff(opts, wsPath, pluginsDir)
	}

	if certificates.Has(opts.WorkspaceDir) {
		log.Info(subsectionHeader(t.Msg("setup_header_certs")))
		log.Info(t.Msg("setup_certs_will_install"))
		if list, err := certificates.List(opts.WorkspaceDir); err == nil {
			for _, name := range list {
				log.Infof("  - %s", name)
			}
		}
	}

	log.Info(t.Msg("setup_gen_all"))
	if err := opts.Generator.GenerateAll(wsPath, pluginsDir, opts.WorkspaceDir, opts.Stderr); err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	log.Info(t.Msg("setup_gen_env"))
	if err := writeEnv(opts.WorkspaceDir, ws, uid, gid, dockerGID); err != nil {
		return fmt.Errorf("write env: %w", err)
	}

	if err := copyShellrcCustomIfMissing(opts.WorkspaceDir, log, t); err != nil {
		log.Warnf("%v", err)
	}

	if err := ensureHomeFiles(ws, log, t); err != nil {
		return fmt.Errorf("ensure home files: %w", err)
	}

	if !opts.NoClone {
		logFn := func(level, msg string) {
			log.Infof("[%s] %s", level, msg)
		}
		if _, err := opts.Cloner.CloneAll(opts.WorkspaceDir, logFn); err != nil {
			log.Warnf("clone repositories: %v", err)
		}
	}

	printResult(log, t, ws, uid, gid, dockerGID, opts.WorkspaceDir)
	return nil
}

func defaults(o *Options) {
	if o.Stdin == nil {
		o.Stdin = os.Stdin
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.Logger == nil {
		o.Logger = logx.New(o.Stdout, o.Stderr)
	}
	if o.Selector == nil {
		o.Selector = tui.HuhSelector{}
	}
	if o.Generator == nil {
		o.Generator = execGenerator{}
	}
	if o.Cloner == nil {
		o.Cloner = defaultCloner{}
	}
	if o.GIDDetector == nil {
		o.GIDDetector = defaultGIDDetector{}
	}
}

type execGenerator struct{}

// GenerateAll runs the LoadContext → BuildArtifacts → WriteArtifacts
// pipeline directly. It used to subprocess into the `generate-all`
// cobra command, which was retired together with the other docker-
// compose-wrapper verbs in v0.2.0.
func (execGenerator) GenerateAll(wsPath, pluginsDir, outputDir string, stderr io.Writer) error {
	ctx, err := generatecli.LoadContext(wsPath, pluginsDir, stderr)
	if err != nil {
		return err //nolint:wrapcheck // caller wraps.
	}
	arts, err := generatecli.BuildArtifacts(ctx, pluginsDir, stderr)
	if err != nil {
		return err //nolint:wrapcheck // caller wraps.
	}
	return generatecli.WriteArtifacts(arts, outputDir) //nolint:wrapcheck // caller wraps.
}

type defaultCloner struct{}

// CloneAll clones every companion repository declared by the workspace.
// The error wrap is left to the caller (Run) to avoid double-prefixing.
func (defaultCloner) CloneAll(scriptDir string, log func(level, msg string)) (repositories.CloneSummary, error) {
	return repositories.CloneAll(exec.New(), scriptDir, log) //nolint:wrapcheck // caller wraps.
}
