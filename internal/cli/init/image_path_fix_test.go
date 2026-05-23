//nolint:testpackage // exercises unexported imagePathFix helpers.
package initcli

import (
	"sort"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
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
