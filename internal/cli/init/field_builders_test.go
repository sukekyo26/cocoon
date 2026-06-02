//nolint:testpackage // exercises unexported path-fix formatters and field builders.
package initcli

import (
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/i18n"
)

// TestFormatPathFixEntries pins the [container.shell.env] preview layout:
// a 2-space indent, keys left-padded to the widest entry, and the value
// quoted with %q. Empty input yields an empty string (no header line).
func TestFormatPathFixEntries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		entries []pathFixEnvEntry
		want    string
	}{
		{name: "empty", entries: nil, want: ""},
		{
			name:    "single entry needs no padding",
			entries: []pathFixEnvEntry{{Key: "PATH", Value: "$HOME/.local/bin:$PATH"}},
			want:    `  PATH = "$HOME/.local/bin:$PATH"`,
		},
		{
			name: "multi aligns keys to the widest",
			entries: []pathFixEnvEntry{
				{Key: "NPM_CONFIG_PREFIX", Value: "$HOME/.npm-global"},
				{Key: "PATH", Value: "$HOME/.npm-global/bin:$PATH"},
			},
			// PATH (4) is padded to NPM_CONFIG_PREFIX's width (17) → 13 spaces.
			want: "  NPM_CONFIG_PREFIX = \"$HOME/.npm-global\"\n" +
				"  PATH" + strings.Repeat(" ", 13) + " = \"$HOME/.npm-global/bin:$PATH\"",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatPathFixEntries(tc.entries); got != tc.want {
				t.Errorf("formatPathFixEntries:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestFormatPathFixVolumes mirrors TestFormatPathFixEntries for the
// [volumes] section so the two preview blocks line up visually.
func TestFormatPathFixVolumes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		volumes []pathFixVolume
		want    string
	}{
		{name: "empty", volumes: nil, want: ""},
		{
			name:    "single entry needs no padding",
			volumes: []pathFixVolume{{Name: "go", Path: "/home/${USERNAME}/go"}},
			want:    `  go = "/home/${USERNAME}/go"`,
		},
		{
			name: "multi aligns names to the widest",
			volumes: []pathFixVolume{
				{Name: "npm-global", Path: "/home/${USERNAME}/.npm-global"},
				{Name: "npm", Path: "/home/${USERNAME}/.npm"},
			},
			// npm (3) is padded to npm-global's width (10) → 7 spaces.
			want: "  npm-global = \"/home/${USERNAME}/.npm-global\"\n" +
				"  npm" + strings.Repeat(" ", 7) + " = \"/home/${USERNAME}/.npm\"",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatPathFixVolumes(tc.volumes); got != tc.want {
				t.Errorf("formatPathFixVolumes:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestFormatPathFixPreview covers both structural branches: an image with
// volumes emits the [volumes] block after a blank-line separator, while an
// image without volumes (python's shape) omits it entirely.
func TestFormatPathFixPreview(t *testing.T) {
	t.Parallel()

	t.Run("with volumes", func(t *testing.T) {
		t.Parallel()
		fix := imagePathFix{
			Entries: []pathFixEnvEntry{{Key: "PATH", Value: "$HOME/go/bin:$PATH"}},
			Volumes: []pathFixVolume{{Name: "go", Path: "/home/${USERNAME}/go"}},
			Command: "go install <pkg>@latest",
		}
		want := "  [container.shell.env]\n" +
			`  PATH = "$HOME/go/bin:$PATH"` +
			"\n\n  [volumes]\n" +
			`  go = "/home/${USERNAME}/go"`
		if got := formatPathFixPreview(fix); got != want {
			t.Errorf("formatPathFixPreview (with volumes):\n got %q\nwant %q", got, want)
		}
	})

	t.Run("without volumes omits the [volumes] block", func(t *testing.T) {
		t.Parallel()
		fix := imagePathFix{
			Entries: []pathFixEnvEntry{{Key: "PATH", Value: "$HOME/.local/bin:$PATH"}},
			Volumes: nil,
			Command: "pip install --user <pkg>",
		}
		got := formatPathFixPreview(fix)
		want := "  [container.shell.env]\n" + `  PATH = "$HOME/.local/bin:$PATH"`
		if got != want {
			t.Errorf("formatPathFixPreview (no volumes):\n got %q\nwant %q", got, want)
		}
		if strings.Contains(got, "[volumes]") {
			t.Errorf("preview emitted a [volumes] block for a volume-less fix: %q", got)
		}
	})
}

// TestValidateSudoPassword pins the prompt's accept/reject contract: a blank
// or whitespace-only value is rejected as empty, a value containing a newline
// or carriage return is rejected as multiline (the build reads only the first
// .env.local line), and an ordinary single-line value — including the `:`/`=`
// chars the build tolerates — is accepted. The returned message distinguishes
// the two rejection reasons.
func TestValidateSudoPassword(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	empty := cat.Msg("init_err_sudo_password_empty")
	multiline := cat.Msg("init_err_sudo_password_multiline")
	cases := []struct {
		name    string
		input   string
		wantMsg string // "" = accept (nil error)
	}{
		{"valid simple", "s3cr3t", ""},
		{"valid with colon and equals", "p@ss:w=rd/x", ""},
		{"valid with internal spaces", "two words", ""},
		{"empty", "", empty},
		{"spaces only", "   ", empty},
		{"newline only trims to empty", "\n", empty},
		{"embedded LF", "line1\nline2", multiline},
		{"trailing LF", "pw\n", multiline},
		{"leading LF with content", "\npw", multiline},
		{"internal CR", "pw\rmore", multiline},
		{"CRLF with content", "pw\r\n", multiline},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateSudoPassword(cat, tc.input)
			switch {
			case tc.wantMsg == "":
				if err != nil {
					t.Errorf("validateSudoPassword(%q) = %v, want nil", tc.input, err)
				}
			case err == nil:
				t.Errorf("validateSudoPassword(%q) = nil, want %q", tc.input, tc.wantMsg)
			case err.Error() != tc.wantMsg:
				t.Errorf("validateSudoPassword(%q) = %q, want %q", tc.input, err.Error(), tc.wantMsg)
			}
		})
	}
}

// TestImagePathFixConfirm_BuildsForFixImages is a construction smoke test:
// huh.Confirm exposes no stable getter for its title/description, so this
// only confirms the builder returns a non-nil field for every fixable
// image. It also exercises formatPathFixPreview (spliced into the
// description) for both the with-volumes and volume-less shapes.
func TestImagePathFixConfirm_BuildsForFixImages(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	for _, image := range []string{"node", "python", "golang", "rust", "denoland/deno"} {
		image := image
		t.Run(image, func(t *testing.T) {
			t.Parallel()
			var target bool
			if c := imagePathFixConfirm(cat, image, &target); c == nil {
				t.Fatalf("imagePathFixConfirm(%q) returned nil", image)
			}
		})
	}
}
