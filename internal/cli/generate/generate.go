// Package generatecli implements the in-process generation pipeline used by
// the `cocoon gen` command. It loads workspace.toml + the enabled plugin
// TOMLs once and produces the generated artifacts (Dockerfile,
// docker-compose.yml, devcontainer.json when enabled, docker-entrypoint.sh,
// manage.sh, .env) under .devcontainer/. Callers are responsible for atomic
// placement.
package generatecli

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/fsx"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/compose"
	"github.com/sukekyo26/cocoon/internal/generate/devcontainerjson"
	"github.com/sukekyo26/cocoon/internal/generate/dockerfile"
	"github.com/sukekyo26/cocoon/internal/generate/envfile"
	"github.com/sukekyo26/cocoon/internal/plugin"
	"github.com/sukekyo26/cocoon/internal/warn"
)

// Artifact is one generated file. A zero Mode falls back to 0o644.
type Artifact struct {
	Rel  string
	Body string
	Mode fs.FileMode
}

// LoadContext returns a WorkspaceContext ready for BuildArtifacts after
// running plugin conflict checks. pluginsPathHint decorates "plugin not
// found" warnings; pass "" when the source has no on-disk anchor. Non-fatal
// diagnostics are collected into sink for the caller to drain and localize.
func LoadContext(
	wsPath string,
	pluginsFS fs.FS,
	pluginsPathHint string,
	sink *warn.Sink,
) (*generate.WorkspaceContext, error) {
	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		return nil, clihelpers.FailureWrap(err, "")
	}
	plugins, err := plugin.LoadEnabledFromFS(pluginsFS, ws.Plugins.Enable, sink, pluginsPathHint)
	if err != nil {
		return nil, clihelpers.FailureWrap(err, "")
	}
	if cerr := plugin.CheckConflicts(plugins); cerr != nil {
		return nil, clihelpers.FailureWrap(cerr, "")
	}
	// Lock is left nil here so callers that overwrite cocoon.lock (e.g.
	// `cocoon lock`) are not blocked by a malformed existing lock; `cocoon gen`
	// loads + attaches it via loadGenContext.
	return &generate.WorkspaceContext{
		WS:         ws,
		PluginsFS:  pluginsFS,
		ProjectDir: filepath.Dir(wsPath),
		Plugins:    plugins,
		Lock:       nil,
		Warnings:   sink,
	}, nil
}

// LoadWorkspaceContext assembles the layered plugin FS (embedded < user <
// project) for the workspace at wsPath and loads it via LoadContext, so
// `cocoon gen` and `cocoon lock` resolve plugins identically. The returned
// context has a nil Lock; callers that consume cocoon.lock attach it.
func LoadWorkspaceContext(wsPath string) (*generate.WorkspaceContext, error) {
	embedded, err := plugin.CatalogFS()
	if err != nil {
		return nil, clihelpers.FailureWrap(err, "")
	}
	userDir, err := plugin.UserPluginsDir()
	if err != nil {
		return nil, clihelpers.FailureWrap(err, "")
	}
	layered := plugin.NewLayeredFS(embedded, userDir, plugin.ProjectPluginsDir(wsPath))
	// Record "plugin overridden by <source>" notices into the same sink the
	// loader fills, so DrainWarnings surfaces them.
	sink := warn.New()
	layered.LogOverrides(sink)
	//nolint:wrapcheck // LoadContext already wraps failures in clihelpers.ErrFailure.
	return LoadContext(wsPath, layered, "", sink)
}

// BuildArtifacts produces the in-memory list of generated files
// (docker-compose.yml, Dockerfile, devcontainer.json when enabled,
// docker-entrypoint.sh, manage.sh, .env) for the given loaded
// WorkspaceContext.
func BuildArtifacts(ctx *generate.WorkspaceContext) ([]Artifact, error) {
	arts := make([]Artifact, 0, 6)

	body, err := compose.Generate(ctx, compose.Options{Plugins: ctx.Plugins, Warnings: ctx.Warnings})
	if err != nil {
		return nil, clihelpers.FailureWrap(err, "err_generate_compose")
	}
	arts = append(arts, Artifact{Rel: ".devcontainer/docker-compose.yml", Body: body, Mode: 0})

	body, err = dockerfile.Generate(ctx, dockerfile.Options{
		WorkspaceRoot: ctx.ProjectDir,
		RepoDir:       "",
		Plugins:       ctx.Plugins,
		Warnings:      ctx.Warnings,
	})
	if err != nil {
		return nil, clihelpers.FailureWrap(err, "err_generate_dockerfile")
	}
	arts = append(arts, Artifact{Rel: ".devcontainer/Dockerfile", Body: body, Mode: 0})

	if ctx.WS.Workspace.DevContainerOrDefault() {
		body, err = devcontainerjson.Generate(ctx)
		if err != nil {
			return nil, clihelpers.FailureWrap(err, "err_generate_devcontainerjson")
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
		return nil, clihelpers.FailureWrap(err, "err_generate_envfile")
	}
	arts = append(arts, Artifact{Rel: ".devcontainer/.env", Body: envBody, Mode: 0o600})
	// NB: in password sudo mode .devcontainer/.gitignore is NOT a generated
	// artifact — WriteArtifacts overwrites artifacts, which would clobber a
	// user-managed .gitignore. `cocoon gen` upserts the .env.local ignore line
	// host-side instead (ensureSudoGitignore), preserving existing rules.
	return arts, nil
}

// WriteArtifacts atomically lays the artifact set down under outDir. Each
// artifact uses its own Mode (defaulting to 0o644 when unset).
func WriteArtifacts(arts []Artifact, outDir string) error {
	for _, a := range arts {
		target := filepath.Join(outDir, a.Rel)
		if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); mkErr != nil {
			return clihelpers.FailureWrap(mkErr, "err_generate_mkdir", target)
		}
		mode := a.Mode
		if mode == 0 {
			mode = 0o644
		}
		if wErr := fsx.AtomicWriteFile(target, []byte(a.Body), mode); wErr != nil {
			return clihelpers.FailureWrap(wErr, "err_generate_write", target)
		}
	}
	return nil
}
