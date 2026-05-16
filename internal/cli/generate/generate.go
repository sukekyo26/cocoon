// Package generatecli implements the in-process generation pipeline used by
// the `cocoon gen` command. It loads workspace.toml + the enabled plugin
// TOMLs once and produces the generated artifacts (Dockerfile,
// docker-compose.yml, devcontainer.json when enabled, docker-entrypoint.sh,
// manage.sh, .env) under .devcontainer/. Callers are responsible for atomic
// placement.
package generatecli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/fsx"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/compose"
	"github.com/sukekyo26/cocoon/internal/generate/devcontainerjson"
	"github.com/sukekyo26/cocoon/internal/generate/dockerfile"
	"github.com/sukekyo26/cocoon/internal/generate/envfile"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// ErrUsage signals a usage error (missing argument). Maps to exit code 2.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure. Maps to exit code 1.
var ErrFailure = errors.New("failure")

// Artifact is one generated file. A zero Mode falls back to 0o644.
type Artifact struct {
	Rel  string
	Body string
	Mode fs.FileMode
}

// LoadContext returns a WorkspaceContext ready for BuildArtifacts after
// running plugin conflict checks. pluginsPathHint decorates "plugin not
// found" warnings; pass "" when the source has no on-disk anchor.
func LoadContext(
	wsPath string,
	pluginsFS fs.FS,
	pluginsPathHint string,
	stderr io.Writer,
) (*generate.WorkspaceContext, error) {
	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailure, err)
	}
	warnW := logx.YellowWriter(stderr)
	plugins, err := plugin.LoadEnabledFromFS(pluginsFS, ws.Plugins.Enable, warnW, pluginsPathHint)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailure, err)
	}
	if cerr := plugin.CheckConflicts(plugins); cerr != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailure, cerr)
	}
	return &generate.WorkspaceContext{
		WS:         ws,
		PluginsFS:  pluginsFS,
		ProjectDir: filepath.Dir(wsPath),
		Plugins:    plugins,
		Warnings:   warnW,
	}, nil
}

// BuildArtifacts produces the in-memory list of generated files
// (docker-compose.yml, Dockerfile, devcontainer.json when enabled,
// docker-entrypoint.sh, manage.sh, .env) for the given loaded
// WorkspaceContext.
func BuildArtifacts(ctx *generate.WorkspaceContext, stderr io.Writer) ([]Artifact, error) {
	arts := make([]Artifact, 0, 6)
	warnW := logx.YellowWriter(stderr)

	body, err := compose.Generate(ctx, compose.Options{Plugins: ctx.Plugins, Warnings: warnW})
	if err != nil {
		return nil, fmt.Errorf("%w: compose: %w", ErrFailure, err)
	}
	arts = append(arts, Artifact{Rel: ".devcontainer/docker-compose.yml", Body: body, Mode: 0})

	body, err = dockerfile.Generate(ctx, dockerfile.Options{
		WorkspaceRoot: ctx.ProjectDir,
		RepoDir:       "",
		Plugins:       ctx.Plugins,
		Warnings:      warnW,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: dockerfile: %w", ErrFailure, err)
	}
	arts = append(arts, Artifact{Rel: ".devcontainer/Dockerfile", Body: body, Mode: 0})

	if ctx.WS.Workspace.DevContainerOrDefault() {
		body, err = devcontainerjson.Generate(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: devcontainer.json: %w", ErrFailure, err)
		}
		arts = append(arts, Artifact{Rel: ".devcontainer/devcontainer.json", Body: body, Mode: 0})
	}

	arts = append(arts, Artifact{
		Rel:  ".devcontainer/docker-entrypoint.sh",
		Body: dockerfile.EntrypointScript(),
		Mode: 0o755,
	})

	arts = append(arts, Artifact{
		Rel:  ".devcontainer/manage.sh",
		Body: generate.ManageScript(),
		Mode: 0o755,
	})

	envBody, err := envfile.Generate(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: envfile: %w", ErrFailure, err)
	}
	arts = append(arts, Artifact{Rel: ".devcontainer/.env", Body: envBody, Mode: 0o600})
	return arts, nil
}

// WriteArtifacts atomically lays the artifact set down under outDir. Each
// artifact uses its own Mode (defaulting to 0o644 when unset).
func WriteArtifacts(arts []Artifact, outDir string) error {
	for _, a := range arts {
		target := filepath.Join(outDir, a.Rel)
		if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); mkErr != nil {
			return fmt.Errorf("%w: mkdir %s: %w", ErrFailure, target, mkErr)
		}
		mode := a.Mode
		if mode == 0 {
			mode = 0o644
		}
		if wErr := fsx.AtomicWriteFile(target, []byte(a.Body), mode); wErr != nil {
			return fmt.Errorf("%w: write %s: %w", ErrFailure, target, wErr)
		}
	}
	return nil
}
