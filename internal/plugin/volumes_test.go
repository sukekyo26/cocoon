package plugin_test

import (
	"testing"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

func TestDeriveVolumeName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"/home/${USERNAME}/.aws", "aws"},
		{"/home/${USERNAME}/go", "go"},
		{"/home/${USERNAME}/.config/", "config"},
		{"/var/lib/docker", "docker"},
	}
	for _, tc := range cases {
		if got := plugin.DeriveVolumeName(tc.in); got != tc.want {
			t.Errorf("DeriveVolumeName(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestGetVolumes(t *testing.T) {
	t.Parallel()
	plugs := map[string]*plugin.Plugin{
		"go": {
			Metadata: plugin.Metadata{Name: "Go"},
			Install:  plugin.Install{Volumes: []string{"/home/${USERNAME}/go"}},
		},
		"rust": {
			Metadata: plugin.Metadata{Name: "Rust"},
			Install: plugin.Install{Volumes: []string{
				"/home/${USERNAME}/.cargo",
				"/home/${USERNAME}/.rustup",
			}},
		},
	}
	got := plugin.GetVolumes([]string{"rust", "go"}, plugs)
	if len(got) != 3 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].VolumeName != "cargo" || got[1].VolumeName != "rustup" || got[2].VolumeName != "go" {
		t.Errorf("order/names: %+v", got)
	}
	if got[0].PluginName != "Rust" {
		t.Errorf("plugin name: %+v", got[0])
	}
}
