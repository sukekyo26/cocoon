// Package generatecli implements the `wsd generate-all` subcommand.
//
// generate-all loads workspace.toml + the enabled plugin TOMLs once and
// writes every generated artifact (Dockerfile, docker-compose.yml,
// devcontainer files, the per-shell rc fragment) into <output_dir>. The
// caller is responsible for atomic placement (write to staging dir, then
// move into the final location) — see lib/generators.sh.
package generatecli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/fsx"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/compose"
	"github.com/sukekyo26/cocoon/internal/generate/devcontainerjson"
	"github.com/sukekyo26/cocoon/internal/generate/dockerfile"
	"github.com/sukekyo26/cocoon/internal/generate/shellrc"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// ErrUsage signals a usage error (missing argument). Maps to exit code 2.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure. Maps to exit code 1.
var ErrFailure = errors.New("failure")

// Artifact is one generated file produced by BuildArtifacts. Rel is
// the path relative to the caller-supplied output directory; Body is
// its contents. The type is exported so in-process callers (like
// `cocoon up`) can run the generation pipeline without the cobra
// command surface.
type Artifact struct {
	Rel  string
	Body string
}

// LoadContext loads workspace.toml, the enabled plugins from
// pluginsDir, and runs plugin conflict checks, returning a
// WorkspaceContext ready for BuildArtifacts. Stderr receives
// plugin-loader warnings.
func LoadContext(wsPath, pluginsDir string, stderr io.Writer) (*generate.WorkspaceContext, error) {
	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailure, err)
	}
	plugins, err := plugin.LoadEnabled(pluginsDir, ws.Plugins.Enable, stderr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailure, err)
	}
	if cerr := plugin.CheckConflicts(plugins); cerr != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailure, cerr)
	}
	return &generate.WorkspaceContext{
		WS:         ws,
		PluginsDir: pluginsDir,
		Plugins:    plugins,
		Warnings:   stderr,
	}, nil
}

// BuildArtifacts produces the in-memory list of generated files
// (compose, Dockerfile, devcontainer.json when enabled, shellrc) for
// the given loaded WorkspaceContext.
func BuildArtifacts(ctx *generate.WorkspaceContext, pluginsDir string, stderr io.Writer) ([]Artifact, error) {
	absPlugins, err := filepath.Abs(pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("%w: abs(%s): %w", ErrFailure, pluginsDir, err)
	}
	wsRoot := filepath.Dir(absPlugins)
	arts := make([]Artifact, 0, 5)

	body, err := compose.Generate(ctx, compose.Options{Plugins: ctx.Plugins, Warnings: stderr})
	if err != nil {
		return nil, fmt.Errorf("%w: compose: %w", ErrFailure, err)
	}
	arts = append(arts, Artifact{Rel: ".devcontainer/docker-compose.yml", Body: body})

	body, err = dockerfile.Generate(ctx, dockerfile.Options{
		WorkspaceRoot: wsRoot,
		RepoDir:       "",
		Plugins:       ctx.Plugins,
		Warnings:      stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: dockerfile: %w", ErrFailure, err)
	}
	arts = append(arts, Artifact{Rel: ".devcontainer/Dockerfile", Body: body})

	if ctx.WS.Workspace.DevContainerOrDefault() {
		body, err = devcontainerjson.Generate(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: devcontainer.json: %w", ErrFailure, err)
		}
		arts = append(arts, Artifact{Rel: ".devcontainer/devcontainer.json", Body: body})
	}

	rcRel, body, err := shellrc.Generate(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: shellrc: %w", ErrFailure, err)
	}
	arts = append(arts, Artifact{Rel: rcRel, Body: body})
	return arts, nil
}

// WriteArtifacts atomically lays the artifact set down under outDir.
// Stale per-shell rc fragments left over from a previous shell choice
// are removed so a `bash → zsh → bash` flip does not leave an
// orphaned .zshrc_custom.generated next to the new bash file.
func WriteArtifacts(arts []Artifact, outDir string) error {
	written := make(map[string]struct{}, len(arts))
	for _, a := range arts {
		target := filepath.Join(outDir, a.Rel)
		if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); mkErr != nil {
			return fmt.Errorf("%w: mkdir %s: %w", ErrFailure, target, mkErr)
		}
		if wErr := fsx.AtomicWriteFile(target, []byte(a.Body), 0o644); wErr != nil {
			return fmt.Errorf("%w: write %s: %w", ErrFailure, target, wErr)
		}
		written[a.Rel] = struct{}{}
	}
	// Sweep stale per-shell rc fragments left over from a previous default
	// (e.g. user switched [container.shell].default = "bash" → "zsh"). The
	// user-editable companion files (without .generated suffix) are never
	// touched.
	for _, rel := range shellrc.KnownGeneratedRelPaths() {
		if _, ok := written[rel]; ok {
			continue
		}
		stale := filepath.Join(outDir, rel)
		if _, err := os.Stat(stale); err == nil {
			_ = os.Remove(stale)
		}
	}
	return nil
}
