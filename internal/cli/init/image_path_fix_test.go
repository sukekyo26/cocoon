//nolint:testpackage // exercises unexported imagePathFix helpers.
package initcli

import (
	"sort"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/plugin"
	"github.com/sukekyo26/cocoon/internal/warn"
)

// TestImagePathFixApplies_MatchesLanguageImagesOnly pins that the gate
// covers exactly the language base images (node, python, golang, rust,
// denoland/deno) and nothing else in SupportedImages.
func TestImagePathFixApplies_MatchesLanguageImagesOnly(t *testing.T) {
	t.Parallel()
	wantApply := map[string]bool{
		"ubuntu":        false,
		"debian":        false,
		"node":          true,
		"python":        true,
		"golang":        true,
		"rust":          true,
		"denoland/deno": true,
	}
	for image, want := range wantApply {
		if got := imagePathFixApplies(image); got != want {
			t.Errorf("imagePathFixApplies(%q) = %v, want %v", image, got, want)
		}
	}
	// Catch new images added to SupportedImages without a deliberate
	// decision here — the table above must cover every entry.
	for _, image := range config.SupportedImages {
		if _, ok := wantApply[image]; !ok {
			t.Errorf("SupportedImages added %q without updating imagePathFixApplies expectations", image)
		}
	}
}

// TestImagePathFixFor_KeyShape pins the per-image schema (which env
// keys, which Command). Catches accidental key renames (e.g.
// CARGO_INSTALL_ROOT → CARGO_HOME) and command-name drift in the
// auto-comment.
func TestImagePathFixFor_KeyShape(t *testing.T) {
	t.Parallel()
	cases := []struct {
		image    string
		wantKeys []string
		wantCmd  string
	}{
		{"node", []string{"NPM_CONFIG_PREFIX", "PATH"}, "npm install -g <pkg>"},
		{"python", []string{"PATH"}, "pip install --user <pkg>"},
		{"golang", []string{"PATH"}, "go install <pkg>@latest"},
		{"rust", []string{"CARGO_INSTALL_ROOT", "PATH"}, "cargo install <pkg>"},
		{"denoland/deno", []string{"PATH"}, "deno install <script>"},
	}
	for _, tc := range cases {
		t.Run(tc.image, func(t *testing.T) {
			t.Parallel()
			fix := imagePathFixFor(tc.image)
			if fix.Command != tc.wantCmd {
				t.Errorf("Command = %q, want %q", fix.Command, tc.wantCmd)
			}
			gotKeys := make([]string, len(fix.Entries))
			for i, e := range fix.Entries {
				gotKeys[i] = e.Key
			}
			if !equalStringSlices(gotKeys, tc.wantKeys) {
				t.Errorf("Entry keys = %v, want %v (order matters — workspace.toml emit order)",
					gotKeys, tc.wantKeys)
			}
		})
	}
}

// TestImagePathFixFor_ValuesUseHomeAndPATHReferences pins that every
// value uses $HOME (not /root, not /home/dev) and that PATH entries
// preserve the trailing :$PATH suffix. These shape rules matter because
// shellrc.go emits the values verbatim — a regression here would land
// in user shells unchanged.
func TestImagePathFixFor_ValuesUseHomeAndPATHReferences(t *testing.T) {
	t.Parallel()
	for _, image := range []string{"node", "python", "golang", "rust", "denoland/deno"} {
		fix := imagePathFixFor(image)
		for _, e := range fix.Entries {
			if e.Key == "PATH" {
				if !endsWith(e.Value, ":$PATH") {
					t.Errorf("%s: PATH value %q must end with `:$PATH` to compose with image-set PATH",
						image, e.Value)
				}
			}
			if !startsWithHome(e.Value) {
				t.Errorf("%s: %s value %q must start with $HOME (no hard-coded usernames)",
					image, e.Key, e.Value)
			}
		}
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	// Compare in original order too, so a sort-order mismatch surfaces.
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func startsWithHome(s string) bool {
	const prefix = "$HOME"
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// TestImagePathFix_VolumeNamesDeriveFromPaths pins the contract that each
// pathFixVolume.Name equals plugin.DeriveVolumeName(Path). This guarantees
// the [volumes] keys cocoon writes for image-path-fix line up with how the
// compose generator names plugin volumes — any drift here would emit
// inconsistent compose snapshots depending on whether the user picked the
// image route or the plugin route.
func TestImagePathFix_VolumeNamesDeriveFromPaths(t *testing.T) {
	t.Parallel()
	for image, fix := range imagePathFixes {
		for _, v := range fix.Volumes {
			got := plugin.DeriveVolumeName(v.Path)
			if got != v.Name {
				t.Errorf("%s: pathFixVolume{Name: %q, Path: %q} — DeriveVolumeName=%q, want Name to match",
					image, v.Name, v.Path, got)
			}
		}
	}
}

// TestImagePathFix_VolumePathsAreContainerAbsolute pins that every Path
// is a container-absolute path under /home/${USERNAME}/. Compose mount
// targets must be absolute (`name:/abs/path`), and the `${USERNAME}`
// placeholder is required so the generated YAML works across users.
func TestImagePathFix_VolumePathsAreContainerAbsolute(t *testing.T) {
	t.Parallel()
	const wantPrefix = "/home/${USERNAME}/"
	for image, fix := range imagePathFixes {
		for _, v := range fix.Volumes {
			if len(v.Path) < len(wantPrefix) || v.Path[:len(wantPrefix)] != wantPrefix {
				t.Errorf("%s: Volume %q Path=%q must start with %q",
					image, v.Name, v.Path, wantPrefix)
			}
		}
	}
}

// TestImagePathFix_VolumesMatchPluginCatalog asserts that the volume
// names cocoon emits for an image-path-fix image are a subset of the
// volume names the equivalent catalog plugin would emit. This guarantees
// "image route" and "plugin route" stay structurally equivalent at the
// compose level — adding a new volume to a plugin without mirroring it in
// the matching imagePathFix (or vice versa) breaks the user-visible
// promise that swapping image⇄plugin yields the same compose volumes.
//
// python is intentionally absent from this map: it has no Volumes
// because $HOME/.local is already covered by the reserved `local:`
// named volume, and no `python` plugin exists in the catalog (python is
// apt-installed). denoland/deno → deno, golang → go, etc.
func TestImagePathFix_VolumesMatchPluginCatalog(t *testing.T) {
	t.Parallel()
	// imageToPluginID maps each image-path-fix image to the catalog plugin
	// id that ships the same language toolchain. python is omitted by design.
	imageToPluginID := map[string]string{
		"node":          "node",
		"golang":        "go",
		"rust":          "rust",
		"denoland/deno": "deno",
	}
	src, err := plugin.CatalogFS()
	if err != nil {
		t.Fatalf("plugin.CatalogFS: %v", err)
	}
	ids := make([]string, 0, len(imageToPluginID))
	for _, id := range imageToPluginID {
		ids = append(ids, id)
	}
	plugins, err := plugin.LoadEnabledFromFS(src, ids, warn.New(), "")
	if err != nil {
		t.Fatalf("plugin.LoadEnabledFromFS: %v", err)
	}
	for image, pluginID := range imageToPluginID {
		t.Run(image, func(t *testing.T) {
			t.Parallel()
			p, ok := plugins[pluginID]
			if !ok {
				t.Fatalf("catalog plugin %q not loaded", pluginID)
			}
			pluginVolNames := make(map[string]struct{}, len(p.Install.Volumes))
			for _, path := range p.Install.Volumes {
				pluginVolNames[plugin.DeriveVolumeName(path)] = struct{}{}
			}
			fix := imagePathFixFor(image)
			if len(fix.Volumes) == 0 {
				t.Fatalf("imagePathFixes[%q] has no Volumes — language images must persist their install destinations", image)
			}
			for _, v := range fix.Volumes {
				if _, ok := pluginVolNames[v.Name]; !ok {
					t.Errorf("%s: volume %q not declared by catalog plugin %q (drift between image-path-fix and plugin route)",
						image, v.Name, pluginID)
				}
			}
		})
	}
}

// TestImagePathFix_PythonHasNoVolumes pins that python's image-path-fix
// intentionally carries no Volumes. python's install target ($HOME/.local
// for `pip install --user`) is already covered by cocoon's reserved
// `local:` named volume; adding a redundant entry would collide with the
// compose generator's reservedMountPaths check.
func TestImagePathFix_PythonHasNoVolumes(t *testing.T) {
	t.Parallel()
	if got := imagePathFixFor("python").Volumes; len(got) != 0 {
		t.Errorf("python Volumes = %+v, want empty (covered by reserved `local:` volume)", got)
	}
}

// TestImagePathFix_RustOmitsRustup pins that rust's image-path-fix does
// not volume-mount $HOME/.rustup. The official rust Docker image keeps
// rustup state at /usr/local/rustup (not $HOME), so a $HOME/.rustup
// volume would persist an empty directory and confuse anyone reading
// the generated compose.
func TestImagePathFix_RustOmitsRustup(t *testing.T) {
	t.Parallel()
	for _, v := range imagePathFixFor("rust").Volumes {
		if v.Path == "/home/${USERNAME}/.rustup" {
			t.Errorf("rust must not volume-mount $HOME/.rustup (rustup state lives at /usr/local/rustup on the rust image)")
		}
	}
}
