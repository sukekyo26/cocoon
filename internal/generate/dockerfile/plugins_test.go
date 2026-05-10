package dockerfile //nolint:testpackage // exercises unexported generatePluginInstalls / userDirsBlockTmpl directly.

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// errFS is a minimal fs.FS that fails every Open with the configured
// error. fs.Stat falls back to opening the file when the underlying FS
// does not implement StatFS, so this is enough to exercise non-ENOENT
// error propagation in fileExistsInFS without needing a real
// permission-denied fixture (hard to set up portably in CI).
type errFS struct{ err error }

func (e errFS) Open(string) (fs.File, error) { return nil, e.err }

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

// TestFileExistsInFS_Contract pins the (bool, error) contract added
// in response to the PR #12 review: "not found" returns (false, nil)
// like os.Stat's ENOENT branch, but every other stat failure
// (permission, I/O, transient FS errors) is propagated up so the
// renderer doesn't silently emit an incomplete Dockerfile.
func TestFileExistsInFS_Contract(t *testing.T) {
	t.Parallel()

	t.Run("file_present_returns_true_nil", func(t *testing.T) {
		t.Parallel()
		fsys := fstest.MapFS{"plugins/foo/install.sh": &fstest.MapFile{Data: []byte("#!/bin/sh\n")}}
		ok, err := fileExistsInFS(fsys, "plugins/foo/install.sh")
		if err != nil || !ok {
			t.Fatalf("got (%v, %v), want (true, nil)", ok, err)
		}
	})

	t.Run("file_missing_returns_false_nil", func(t *testing.T) {
		t.Parallel()
		ok, err := fileExistsInFS(fstest.MapFS{}, "plugins/foo/install.sh")
		if err != nil || ok {
			t.Fatalf("got (%v, %v), want (false, nil)", ok, err)
		}
	})

	t.Run("directory_returns_false_nil", func(t *testing.T) {
		t.Parallel()
		// A directory entry must report "not a regular file" without
		// erroring — buildPluginSnippets relies on this to keep
		// emit-decision logic linear.
		fsys := fstest.MapFS{"plugins/foo/install.sh/marker": &fstest.MapFile{}}
		ok, err := fileExistsInFS(fsys, "plugins/foo/install.sh")
		if err != nil || ok {
			t.Fatalf("got (%v, %v), want (false, nil)", ok, err)
		}
	})

	t.Run("non_notexist_error_propagates", func(t *testing.T) {
		t.Parallel()
		ok, err := fileExistsInFS(errFS{err: fs.ErrPermission}, "plugins/foo/install.sh")
		if ok {
			t.Errorf("ok = true, want false")
		}
		if !errors.Is(err, fs.ErrPermission) {
			t.Fatalf("err = %v, want errors.Is(.., fs.ErrPermission)", err)
		}
	})
}

// TestBuildPluginSnippets_HeredocCollisionFails covers the
// checkHeredocCollision contract: an install script containing a line
// that exactly matches the COCOON_PLUGIN_EOF terminator would silently
// truncate the heredoc at docker build time, so the renderer must
// reject it up front. Three input shapes lock the detection:
//   - a delimiter-only line in the middle of the script (the actual
//     truncation case)
//   - a delimiter as the very last line, no trailing newline (boundary
//     case the renderer normalises elsewhere)
//   - a near-miss (delimiter prefix only) must NOT trip — the helper
//     compares full lines, not substrings, so legitimate scripts that
//     reference the literal in a comment or echo do not regress.
func TestBuildPluginSnippets_HeredocCollisionFails(t *testing.T) {
	t.Parallel()

	good := "set -e\n# mentions COCOON_PLUGIN_EOF in a comment, fine\necho hi\n"
	collidingMiddle := "set -e\nCOCOON_PLUGIN_EOF\necho hi\n"
	collidingTrailing := "set -e\necho hi\nCOCOON_PLUGIN_EOF"

	cases := []struct {
		name      string
		body      string
		wantError bool
	}{
		{name: "no_collision_passes", body: good, wantError: false},
		{name: "delimiter_in_middle_fails", body: collidingMiddle, wantError: true},
		{name: "delimiter_as_last_line_fails", body: collidingTrailing, wantError: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			plugins := map[string]*plugin.Plugin{
				"colliding": {
					Metadata: plugin.Metadata{Name: "Colliding"}, //nolint:exhaustruct // unused
					Install:  plugin.Install{},                   //nolint:exhaustruct // unused
				}, //nolint:exhaustruct // Apt / Version unused
			}
			pluginsDir := t.TempDir()
			seedPluginInstallBody(t, pluginsDir, "colliding", tc.body)

			_, err := generatePluginInstalls(
				plugins, []string{"colliding"}, os.DirFS(pluginsDir), nil,
				map[string]config.PluginVersionOverride{},
				&bytes.Buffer{},
				shellEnv{rcFileAbs: "/home/${USERNAME}/.bashrc", rcSyntax: "posix", loginShell: "bash"},
			)
			if tc.wantError {
				if !errors.Is(err, ErrHeredocCollision) {
					t.Fatalf("err = %v, want errors.Is(.., ErrHeredocCollision)", err)
				}
				if !strings.Contains(err.Error(), "COCOON_PLUGIN_EOF") {
					t.Errorf("error message must name the colliding literal: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// seedPluginInstallBody is the body-controlled twin of seedPluginInstall:
// callers pass the literal install.sh contents so heredoc-collision
// fixtures (and any future shape-sensitive tests) don't have to fight
// the default "#!/usr/bin/env bash\n" stub.
func seedPluginInstallBody(t *testing.T, pluginsDir, id, body string) {
	t.Helper()
	dir := filepath.Join(pluginsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte("# stub\n"), 0o600); err != nil {
		t.Fatalf("write plugin.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "install.sh"), []byte(body), 0o600); err != nil {
		t.Fatalf("write install.sh: %v", err)
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
