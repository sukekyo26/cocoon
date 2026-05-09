package compose_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/compose"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

func TestGenerate_RejectsReservedCustomVolume(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"local", "cocoon"} {
		ws := &config.Workspace{
			Container: config.ContainerSpec{
				ServiceName: "dev",
				Username:    "u",
				Os:          "ubuntu",
				OsVersion:   "26.04",
			},
			Volumes: map[string]string{
				name: "/home/u/some-path",
			},
		}
		ctx := &generate.WorkspaceContext{WS: ws}
		var warns bytes.Buffer
		_, err := compose.Generate(ctx, compose.Options{Warnings: &warns})
		if !errors.Is(err, compose.ErrVolumeNameConflict) {
			t.Errorf("custom volume %q: expected ErrVolumeNameConflict, got %v", name, err)
		}
	}
}

func TestGenerate_RejectsCustomVolumeOnReservedMountPath(t *testing.T) {
	t.Parallel()
	// A volume with an allowed name but pointing at a path cocoon
	// already mounts unconditionally would otherwise emit two compose
	// volume entries for the same target — docker compose v2 fails or
	// silently overrides one of them. Reject at gen time instead.
	for _, path := range []string{
		"/home/${USERNAME}/.local",
		"/home/${USERNAME}/.cocoon",
	} {
		ws := &config.Workspace{
			Container: config.ContainerSpec{
				ServiceName: "dev",
				Username:    "u",
				Os:          "ubuntu",
				OsVersion:   "26.04",
			},
			Volumes: map[string]string{
				"my_state": path,
			},
		}
		ctx := &generate.WorkspaceContext{WS: ws}
		var warns bytes.Buffer
		_, err := compose.Generate(ctx, compose.Options{Warnings: &warns})
		if !errors.Is(err, compose.ErrVolumeNameConflict) {
			t.Errorf("custom volume target %q: expected ErrVolumeNameConflict, got %v", path, err)
		}
	}
}

func TestGenerate_RejectsPluginVolumeOnReservedMountPath(t *testing.T) {
	t.Parallel()
	// Same collision risk as the custom-volume case but for plugin
	// volumes. The two reserved-path checks live in different loops in
	// mergeVolumes, so cover both.
	for _, path := range []string{
		"/home/${USERNAME}/.local",
		"/home/${USERNAME}/.cocoon",
	} {
		ws := &config.Workspace{
			Container: config.ContainerSpec{
				ServiceName: "dev",
				Username:    "u",
				Os:          "ubuntu",
				OsVersion:   "26.04",
			},
			Plugins: config.PluginsSpec{Enable: []string{"badvol"}},
		}
		plugins := map[string]*plugin.Plugin{
			"badvol": {
				Metadata: plugin.Metadata{Name: "badvol"},
				Install:  plugin.Install{Volumes: []string{path}},
			},
		}
		ctx := &generate.WorkspaceContext{WS: ws, Plugins: plugins}
		var warns bytes.Buffer
		_, err := compose.Generate(ctx, compose.Options{Plugins: plugins, Warnings: &warns})
		if !errors.Is(err, compose.ErrVolumeNameConflict) {
			t.Errorf("plugin volume target %q: expected ErrVolumeNameConflict, got %v", path, err)
		}
	}
}
