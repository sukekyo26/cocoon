package compose_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/compose"
	"github.com/sukekyo26/cocoon/internal/plugin"
	"github.com/sukekyo26/cocoon/internal/warn"
)

// TestGenerate_DeduplicatesSharedPluginVolumePathSilently pins the contract in
// mergePluginVolumes: two plugins declaring the same mount path collapse to one
// volume, and gen does so silently. The overlap is owned by the (often
// built-in) plugin authors, not the user, so the old dedup warning was
// unactionable noise — see the removed warn.VolumeDupPlugin.
func TestGenerate_DeduplicatesSharedPluginVolumePathSilently(t *testing.T) {
	t.Parallel()
	const sharedPath = "/home/${USERNAME}/.config"
	ws := &config.Workspace{
		Container: config.ContainerSpec{
			ServiceName:  "dev",
			Username:     "u",
			Image:        "ubuntu",
			ImageVersion: "26.04",
		},
		Plugins: config.PluginsSpec{Enable: []string{"alpha", "beta"}},
	}
	plugins := map[string]*plugin.Plugin{
		"alpha": {
			Metadata: plugin.Metadata{Name: "alpha"},
			Install:  plugin.Install{Volumes: []string{sharedPath}},
		},
		"beta": {
			Metadata: plugin.Metadata{Name: "beta"},
			Install:  plugin.Install{Volumes: []string{sharedPath}},
		},
	}
	ctx := &generate.WorkspaceContext{WS: ws, Plugins: plugins}
	warns := warn.New()

	got, err := compose.Generate(ctx, compose.Options{Plugins: plugins, Warnings: warns})
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	// Silently: these plugins share one path and declare nothing else that
	// warns, so the sink must be empty.
	if all := warns.All(); len(all) != 0 {
		t.Errorf("Generate() emitted %d diagnostic(s), want 0: %+v", len(all), all)
	}

	// One volume: the shared path derives the name `config`, which must appear
	// exactly once as a top-level volume definition and once as a service mount.
	wantVol := plugin.DeriveVolumeName(sharedPath)
	if n := strings.Count(got, "\n  "+wantVol+":\n"); n != 1 {
		t.Errorf("volume definition %q appears %d time(s), want 1:\n%s", wantVol, n, got)
	}
	if n := strings.Count(got, wantVol+":"+sharedPath); n != 1 {
		t.Errorf("mount %q appears %d time(s), want 1:\n%s", wantVol+":"+sharedPath, n, got)
	}
}
