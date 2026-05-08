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
	"github.com/sukekyo26/cocoon/internal/generate/devcontainercompose"
	"github.com/sukekyo26/cocoon/internal/generate/devcontainerjson"
	"github.com/sukekyo26/cocoon/internal/generate/dockerfile"
	"github.com/sukekyo26/cocoon/internal/generate/shellrc"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// ErrUsage signals a usage error (missing argument). Maps to exit code 2.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure. Maps to exit code 1.
var ErrFailure = errors.New("failure")

type artifact struct {
	rel  string
	body string
}

func loadContext(wsPath, pluginsDir string, stderr io.Writer) (*generate.WorkspaceContext, error) {
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

func buildArtifacts(ctx *generate.WorkspaceContext, pluginsDir string, stderr io.Writer) ([]artifact, error) {
	absPlugins, err := filepath.Abs(pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("%w: abs(%s): %w", ErrFailure, pluginsDir, err)
	}
	wsRoot := filepath.Dir(absPlugins)
	arts := make([]artifact, 0, 5)

	body, err := compose.Generate(ctx, compose.Options{Plugins: ctx.Plugins, Warnings: stderr})
	if err != nil {
		return nil, fmt.Errorf("%w: compose: %w", ErrFailure, err)
	}
	arts = append(arts, artifact{rel: "docker-compose.yml", body: body})

	body, err = dockerfile.Generate(ctx, dockerfile.Options{
		WorkspaceRoot: wsRoot,
		RepoDir:       "",
		Plugins:       ctx.Plugins,
		Warnings:      stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: dockerfile: %w", ErrFailure, err)
	}
	arts = append(arts, artifact{rel: "Dockerfile", body: body})

	body, err = devcontainerjson.Generate(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: devcontainer.json: %w", ErrFailure, err)
	}
	arts = append(arts, artifact{rel: ".devcontainer/devcontainer.json", body: body})

	body, err = devcontainercompose.Generate(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: devcontainer compose: %w", ErrFailure, err)
	}
	arts = append(arts, artifact{rel: ".devcontainer/docker-compose.yml", body: body})

	rcRel, body, err := shellrc.Generate(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: shellrc: %w", ErrFailure, err)
	}
	arts = append(arts, artifact{rel: rcRel, body: body})
	return arts, nil
}

func writeArtifacts(arts []artifact, outDir string) error {
	written := make(map[string]struct{}, len(arts))
	for _, a := range arts {
		target := filepath.Join(outDir, a.rel)
		if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); mkErr != nil {
			return fmt.Errorf("%w: mkdir %s: %w", ErrFailure, target, mkErr)
		}
		if wErr := fsx.AtomicWriteFile(target, []byte(a.body), 0o644); wErr != nil {
			return fmt.Errorf("%w: write %s: %w", ErrFailure, target, wErr)
		}
		written[a.rel] = struct{}{}
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
