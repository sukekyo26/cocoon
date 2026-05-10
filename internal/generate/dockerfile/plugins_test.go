package dockerfile //nolint:testpackage // exercises unexported generatePluginInstalls / userDirsBlockTmpl directly.

import (
	"bytes"
	"errors"
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

// TestGeneratePluginInstalls_EnvOnlyPluginEmitsEnv covers the case
// where a plugin defines [install.env] but ships no install.sh. Before
// the fix the env block sat inside the hasInstall branch and was
// silently dropped, even though buildPluginSnippets reported the
// plugin as non-empty. The output must contain the rendered ENV lines
// in [install.env] declaration order.
func TestGeneratePluginInstalls_EnvOnlyPluginEmitsEnv(t *testing.T) {
	t.Parallel()

	plugins := map[string]*plugin.Plugin{
		"env-only": {
			Metadata: plugin.Metadata{Name: "Env Only"}, //nolint:exhaustruct // unused metadata fields
			Install: plugin.Install{ //nolint:exhaustruct // Volumes / UserDirs / BuildArgs unused
				Env: map[string]string{
					"PATH":   "/opt/env-only/bin:$PATH",
					"FOO":    "bar",
					"PYPATH": "/opt/env-only/lib",
				},
			},
		}, //nolint:exhaustruct // Apt / Version not exercised by this test
	}
	enabled := []string{"env-only"}
	pluginsDir := t.TempDir()
	seedEnvOnlyPlugin(t, pluginsDir, "env-only")

	out, err := generatePluginInstalls(
		plugins, enabled, os.DirFS(pluginsDir), nil,
		map[string]config.PluginVersionOverride{},
		&bytes.Buffer{},
		shellEnv{rcFileAbs: "/home/${USERNAME}/.bashrc", rcSyntax: "posix", loginShell: "bash"},
	)
	if err != nil {
		t.Fatalf("generatePluginInstalls: %v", err)
	}

	for _, want := range []string{
		`ENV PATH="/opt/env-only/bin:$PATH"`,
		`ENV FOO="bar"`,
		`ENV PYPATH="/opt/env-only/lib"`,
		"# Configure Env Only (env)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, out)
		}
	}
	// And the env-only snippet must NOT introduce a stray RUN bash heredoc
	// (that would imply the renderer emitted a script for a plugin that
	// has no script).
	if strings.Contains(out, "bash <<'COCOON_PLUGIN_EOF'") {
		t.Errorf("env-only plugin should not produce a bash heredoc:\n%s", out)
	}
}

// TestGeneratePluginInstalls_NilPluginsFSFailsFast pins down the
// fail-fast contract added after PR #12 review: a misconfigured
// WorkspaceContext (PluginsFS = nil) used to make every fileExistsInFS
// call silently report "no install.sh", emitting a Dockerfile that
// ignored the entire plugin set. Now generatePluginInstalls bails
// with plugin.ErrNilPluginsFS the moment any enabled plugin is also
// loaded into the plugins map.
//
// Two negative-control cases ensure the check is *narrow*:
//   - empty enable list — nil PluginsFS is fine when nothing needs it.
//   - enabled-but-not-loaded id — the existing "warn and skip" path
//     for a missing plugin TOML must still work, so the error fires
//     only when there is an actual plugin to render.
func TestGeneratePluginInstalls_NilPluginsFSFailsFast(t *testing.T) {
	t.Parallel()

	loaded := map[string]*plugin.Plugin{
		"loaded": {
			Metadata: plugin.Metadata{Name: "Loaded"}, //nolint:exhaustruct // unused fields
			Install:  plugin.Install{},                //nolint:exhaustruct // unused fields
		}, //nolint:exhaustruct // Apt / Version not exercised by this test
	}
	cases := []struct {
		name      string
		plugins   map[string]*plugin.Plugin
		enabled   []string
		wantError bool
	}{
		{
			name:      "loaded_plugin_with_nil_fs_errors",
			plugins:   loaded,
			enabled:   []string{"loaded"},
			wantError: true,
		},
		{
			name:      "empty_enable_list_with_nil_fs_is_fine",
			plugins:   loaded,
			enabled:   nil,
			wantError: false,
		},
		{
			name:      "enabled_but_unloaded_id_with_nil_fs_is_fine",
			plugins:   map[string]*plugin.Plugin{},
			enabled:   []string{"missing"},
			wantError: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := generatePluginInstalls(
				tc.plugins, tc.enabled, nil, nil,
				map[string]config.PluginVersionOverride{},
				&bytes.Buffer{},
				shellEnv{rcFileAbs: "/home/${USERNAME}/.bashrc", rcSyntax: "posix", loginShell: "bash"},
			)
			if tc.wantError {
				if !errors.Is(err, plugin.ErrNilPluginsFS) {
					t.Fatalf("err = %v, want errors.Is(.., plugin.ErrNilPluginsFS)", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// seedEnvOnlyPlugin lays down a plugin.toml whose [install.env] table
// preserves declaration order (PATH, FOO, PYPATH) so the rendered
// output is deterministic. No install.sh / install_user.sh is written,
// which is the whole point of the fixture.
func seedEnvOnlyPlugin(t *testing.T, pluginsDir, id string) {
	t.Helper()
	dir := filepath.Join(pluginsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	body := "[install.env]\n" +
		"PATH = \"/opt/env-only/bin:$PATH\"\n" +
		"FOO = \"bar\"\n" +
		"PYPATH = \"/opt/env-only/lib\"\n"
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write plugin.toml: %v", err)
	}
}
