package dockerfile //nolint:testpackage // exercises unexported generatePluginInstalls / userDirsBlockTmpl directly.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// TestGeneratePluginInstalls_NoRedundantUserToggle pins down the CODE-02 fix:
// when the user-dirs mkdir block and a root-requiring plugin install run
// back-to-back, we must not emit `USER ${USERNAME}` followed immediately by
// `USER root`. The block stays in `USER root` until non-root work begins.
func TestGeneratePluginInstalls_NoRedundantUserToggle(t *testing.T) {
	t.Parallel()

	plugins := map[string]*plugin.Plugin{
		"needs-root": {
			Metadata: plugin.Metadata{Name: "Needs Root"}, //nolint:exhaustruct // unused metadata fields
			Install: plugin.Install{ //nolint:exhaustruct // unused install fields
				RequiresRoot: true,
				UserDirs:     []string{"/home/${USERNAME}/.cache/needs-root"},
			},
		}, //nolint:exhaustruct // Apt / Version not exercised by this test
	}
	enabled := []string{"needs-root"}
	pluginsDir := t.TempDir()
	seedPluginInstall(t, pluginsDir, "needs-root")

	out, err := generatePluginInstalls(
		plugins, enabled, os.DirFS(pluginsDir), nil,
		map[string]config.PluginVersionOverride{},
		&bytes.Buffer{},
		shellEnv{rcFileAbs: "/home/${USERNAME}/.bashrc", rcSyntax: "posix", loginShell: "bash"},
	)
	if err != nil {
		t.Fatalf("generatePluginInstalls: %v", err)
	}

	if strings.Contains(out, "USER ${USERNAME}\nUSER root") {
		t.Errorf("output contains a redundant USER ${USERNAME} -> USER root toggle pair:\n%s", out)
	}
	if !strings.Contains(out, "USER ${USERNAME}") {
		t.Errorf("output missing the closing USER ${USERNAME} switch:\n%s", out)
	}
	if !strings.Contains(out, "# Prepare volume mount directories with correct ownership") {
		t.Errorf("output missing user-dirs block:\n%s", out)
	}
	if !strings.Contains(out, "# Install Needs Root") {
		t.Errorf("output missing root-bucket install snippet:\n%s", out)
	}
}

// TestRenderInstallRun_InlineHeredoc locks the contract documented on
// installRunTmpl: install.sh contents land verbatim under a quoted
// `<<'COCOON_PLUGIN_EOF' … COCOON_PLUGIN_EOF` heredoc, the body has a
// trailing newline normalised, and the bind-mount form is gone.
func TestRenderInstallRun_InlineHeredoc(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		script         []byte
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:   "verbatim_body_with_trailing_newline",
			script: []byte("set -e\necho hi\n"),
			wantContains: []string{
				"bash <<'COCOON_PLUGIN_EOF'\nset -e\necho hi\nCOCOON_PLUGIN_EOF",
			},
		},
		{
			name:   "missing_trailing_newline_is_normalised",
			script: []byte("echo hi"),
			wantContains: []string{
				"bash <<'COCOON_PLUGIN_EOF'\necho hi\nCOCOON_PLUGIN_EOF",
			},
		},
		{
			name:   "dollar_sequences_are_preserved_verbatim",
			script: []byte("echo $HOME ${PIN}\n"),
			wantContains: []string{
				"bash <<'COCOON_PLUGIN_EOF'\necho $HOME ${PIN}\nCOCOON_PLUGIN_EOF",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := renderInstallRun(
				"my-plugin", "# Install Stuff", nil, tc.script,
				false, false, config.PluginVersionOverride{},
				nil,
				shellEnv{
					rcFileAbs:  "/home/${USERNAME}/.bashrc",
					rcSyntax:   "posix",
					loginShell: "bash",
				},
			)
			if strings.Contains(got, "--mount=type=bind,from=plugins") {
				t.Errorf("output still references the old bind-mount build context:\n%s", got)
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q\n--- got ---\n%s", want, got)
				}
			}
			for _, bad := range tc.wantNotContain {
				if strings.Contains(got, bad) {
					t.Errorf("must not contain %q\n--- got ---\n%s", bad, got)
				}
			}
		})
	}
}

func seedPluginInstall(t *testing.T, pluginsDir, id string) {
	t.Helper()
	dir := filepath.Join(pluginsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte("# stub\n"), 0o600); err != nil {
		t.Fatalf("write plugin.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "install.sh"), []byte("#!/usr/bin/env bash\n"), 0o600); err != nil {
		t.Fatalf("write install.sh: %v", err)
	}
}
