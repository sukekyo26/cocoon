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
			},
		}, //nolint:exhaustruct // Apt / Version not exercised by this test
	}
	enabled := []string{"needs-root"}
	pluginsDir := t.TempDir()
	seedPluginInstall(t, pluginsDir, "needs-root")

	out, err := generatePluginInstalls(
		plugins, enabled, os.DirFS(pluginsDir),
		[]string{"/home/${USERNAME}/.cache/needs-root"},
		map[string]config.PluginVersionOverride{},
		nil,
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
			got, err := renderInstallRun(
				"my-plugin", "# Install Stuff", nil, tc.script,
				false, true, false, config.PluginVersionOverride{},
				nil,
				nil,
				"",
				shellEnv{
					rcFileAbs:  "/home/${USERNAME}/.bashrc",
					rcSyntax:   "posix",
					loginShell: "bash",
				},
			)
			if err != nil {
				t.Fatalf("renderInstallRun: %v", err)
			}
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
			Install: plugin.Install{ //nolint:exhaustruct // Volumes / BuildArgs unused
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
		nil,
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

// TestGeneratePluginInstalls_BuildArgsWithoutInstallSh pins the
// install_user.sh + build_args path: when a plugin declares
// build_args but ships only install_user.sh (no install.sh), the
// generator must still emit the matching `ARG <name>` line so the
// per-RUN env prefix `<name>="${<name>}"` resolves to the build-arg
// value. This was previously broken — renderUserInstallSnippet
// passed nil for argLines so ARGs declared by build_args-only
// plugins or install_user.sh-only plugins never reached the
// Dockerfile.
func TestGeneratePluginInstalls_BuildArgsWithoutInstallSh(t *testing.T) {
	t.Parallel()

	plugins := map[string]*plugin.Plugin{
		"user-only-with-arg": {
			Metadata: plugin.Metadata{Name: "User Only With Arg"}, //nolint:exhaustruct // unused metadata fields
			Install: plugin.Install{ //nolint:exhaustruct // Volumes / Env unused
				RequiresRoot: false,
				BuildArgs:    []string{"MY_GID"},
			},
		}, //nolint:exhaustruct // Apt / Version not exercised by this test
	}
	enabled := []string{"user-only-with-arg"}
	pluginsDir := t.TempDir()
	dir := filepath.Join(pluginsDir, "user-only-with-arg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte("# stub\n"), 0o600); err != nil {
		t.Fatalf("write plugin.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "install_user.sh"),
		[]byte("#!/usr/bin/env bash\necho \"gid=${MY_GID}\"\n"), 0o600); err != nil {
		t.Fatalf("write install_user.sh: %v", err)
	}

	out, err := generatePluginInstalls(
		plugins, enabled, os.DirFS(pluginsDir), nil,
		map[string]config.PluginVersionOverride{},
		nil,
		&bytes.Buffer{},
		shellEnv{rcFileAbs: "/home/${USERNAME}/.bashrc", rcSyntax: "posix", loginShell: "bash"},
	)
	if err != nil {
		t.Fatalf("generatePluginInstalls: %v", err)
	}

	for _, want := range []string{
		"# Configure User Only With Arg (user)",
		"\nARG MY_GID\n",
		`MY_GID="${MY_GID}" bash <<'COCOON_PLUGIN_EOF'`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

// TestGeneratePluginInstalls_BuildArgsDeclaredOnce pins down that
// when a plugin has BOTH install.sh and install_user.sh and declares
// build_args, the matching `ARG <name>` line is emitted exactly
// once — next to install.sh — and is NOT redeclared before the
// install_user.sh RUN. ARGs are stage-scoped, so a second
// declaration would be a redundant duplicate.
func TestGeneratePluginInstalls_BuildArgsDeclaredOnce(t *testing.T) {
	t.Parallel()

	plugins := map[string]*plugin.Plugin{
		"both-hooks-with-arg": {
			Metadata: plugin.Metadata{Name: "Both Hooks"}, //nolint:exhaustruct // unused metadata fields
			Install: plugin.Install{ //nolint:exhaustruct // Volumes / Env unused
				RequiresRoot: true,
				BuildArgs:    []string{"SOME_GID"},
			},
		}, //nolint:exhaustruct // Apt / Version not exercised by this test
	}
	enabled := []string{"both-hooks-with-arg"}
	pluginsDir := t.TempDir()
	dir := filepath.Join(pluginsDir, "both-hooks-with-arg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, fixture := range []struct{ name, body string }{
		{"plugin.toml", "# stub\n"},
		{"install.sh", "#!/usr/bin/env bash\necho root=${SOME_GID}\n"},
		{"install_user.sh", "#!/usr/bin/env bash\necho user=${SOME_GID}\n"},
	} {
		if err := os.WriteFile(filepath.Join(dir, fixture.name), []byte(fixture.body), 0o600); err != nil {
			t.Fatalf("write %s: %v", fixture.name, err)
		}
	}

	out, err := generatePluginInstalls(
		plugins, enabled, os.DirFS(pluginsDir), nil,
		map[string]config.PluginVersionOverride{},
		nil,
		&bytes.Buffer{},
		shellEnv{rcFileAbs: "/home/${USERNAME}/.bashrc", rcSyntax: "posix", loginShell: "bash"},
	)
	if err != nil {
		t.Fatalf("generatePluginInstalls: %v", err)
	}

	// ARG line should appear exactly once (next to install.sh) — not
	// redeclared near install_user.sh's RUN.
	if got := strings.Count(out, "\nARG SOME_GID\n"); got != 1 {
		t.Errorf("ARG SOME_GID count: got %d, want 1\n--- got ---\n%s", got, out)
	}
	// The single ARG line must precede the user-configure block so it
	// is in scope for both RUNs (ARG is stage-scoped).
	argIdx := strings.Index(out, "\nARG SOME_GID\n")
	userBlockIdx := strings.Index(out, "# Configure Both Hooks (user)")
	if argIdx < 0 || userBlockIdx < 0 || argIdx >= userBlockIdx {
		t.Errorf("ARG SOME_GID must appear before # Configure Both Hooks (user)\n--- got ---\n%s", out)
	}
	// Both RUN lines must still carry the per-RUN env prefix so bash
	// sees SOME_GID as an env var.
	wantPrefix := `SOME_GID="${SOME_GID}" bash <<'COCOON_PLUGIN_EOF'`
	if got := strings.Count(out, wantPrefix); got != 2 {
		t.Errorf("per-RUN SOME_GID prefix count: got %d, want 2 (one per hook)\n--- got ---\n%s", got, out)
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
				nil,
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

// TestCheckScriptBody pins the checkScriptBody contract: a plugin install
// script is rejected when it has CRLF / bare-CR line endings (ErrCRLFScript)
// or a line that exactly matches the COCOON_PLUGIN_EOF heredoc terminator
// (ErrHeredocCollision). Legitimate scripts — the literal inside a comment,
// a near-miss with leading whitespace, empty or newline-free bodies — pass.
// crlf_delimiter_line locks the check order: a \r-terminated delimiter line
// must surface as ErrCRLFScript, not slip past the exact-line comparison.
func TestCheckScriptBody(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		body    string
		wantErr error
		wantMsg string // substring of err.Error() when wantErr != nil
	}{
		{name: "clean_lf_body", body: "set -e\necho hi\n"},
		{name: "empty_body", body: ""},
		{name: "no_trailing_newline", body: "echo hi"},
		{name: "delimiter_in_comment", body: "# see COCOON_PLUGIN_EOF below\necho hi\n"},
		{name: "delimiter_with_leading_space", body: "echo a\n COCOON_PLUGIN_EOF\necho b\n"},
		{name: "delimiter_exact_line", body: "echo a\nCOCOON_PLUGIN_EOF\necho b\n", wantErr: ErrHeredocCollision, wantMsg: "COCOON_PLUGIN_EOF"},
		{name: "delimiter_as_last_line", body: "echo a\nCOCOON_PLUGIN_EOF", wantErr: ErrHeredocCollision, wantMsg: "COCOON_PLUGIN_EOF"},
		{name: "crlf_mid_script", body: "set -e\r\necho hi\r\n", wantErr: ErrCRLFScript, wantMsg: "LF"},
		{name: "crlf_at_eof", body: "echo hi\r\n", wantErr: ErrCRLFScript, wantMsg: "LF"},
		{name: "bare_cr", body: "set -e\recho hi\n", wantErr: ErrCRLFScript, wantMsg: "LF"},
		{name: "crlf_delimiter_line", body: "set -e\nCOCOON_PLUGIN_EOF\r\necho hi\n", wantErr: ErrCRLFScript, wantMsg: "LF"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := checkScriptBody("my-plugin", []byte(tc.body))
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("checkScriptBody() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("checkScriptBody() = %v, want errors.Is(.., %v)", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), "my-plugin") {
				t.Errorf("error must name the plugin: %v", err)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error must contain %q: %v", tc.wantMsg, err)
			}
		})
	}
}

// TestBuildPluginSnippets_RejectsUnsafeScript covers checkScriptBody wired
// through the full generatePluginInstalls path: an install script that
// cannot be embedded safely in the COCOON_PLUGIN_EOF heredoc is rejected
// up front. The cases lock both failure classes — a bare delimiter line
// (mid-script or trailing) that would truncate the heredoc, and CRLF line
// endings that would corrupt every command — while a near-miss (the
// literal inside a comment) still passes.
func TestBuildPluginSnippets_RejectsUnsafeScript(t *testing.T) {
	t.Parallel()

	good := "set -e\n# mentions COCOON_PLUGIN_EOF in a comment, fine\necho hi\n"
	collidingMiddle := "set -e\nCOCOON_PLUGIN_EOF\necho hi\n"
	collidingTrailing := "set -e\necho hi\nCOCOON_PLUGIN_EOF"
	crlf := "set -e\r\necho hi\r\n"

	cases := []struct {
		name    string
		body    string
		wantErr error
	}{
		{name: "no_collision_passes", body: good, wantErr: nil},
		{name: "delimiter_in_middle_fails", body: collidingMiddle, wantErr: ErrHeredocCollision},
		{name: "delimiter_as_last_line_fails", body: collidingTrailing, wantErr: ErrHeredocCollision},
		{name: "crlf_line_endings_fail", body: crlf, wantErr: ErrCRLFScript},
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
				nil,
				&bytes.Buffer{},
				shellEnv{rcFileAbs: "/home/${USERNAME}/.bashrc", rcSyntax: "posix", loginShell: "bash"},
			)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want errors.Is(.., %v)", err, tc.wantErr)
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

// TestGeneratePluginInstalls_MethodOverride pins that workspace.toml's
// [plugins.methods] map selects which install.<method>.sh is embedded
// and injects the COCOON_INSTALL_METHOD env var into the RUN step.
func TestGeneratePluginInstalls_MethodOverride(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: plugin.Metadata{Name: "tester", URL: "https://example.com/x"},
		Install: plugin.Install{
			RequiresRoot:  false,
			DefaultMethod: "official",
			Methods: map[string]plugin.InstallMethod{
				"official": {Description: "Official"},
				"binary":   {Description: "Binary"},
			},
		},
	}
	plugins := map[string]*plugin.Plugin{"tester": p}
	enabled := []string{"tester"}
	pluginsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pluginsDir, "tester"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, "tester", "install.official.sh"), []byte("echo OFFICIAL\n"), 0o600); err != nil {
		t.Fatalf("write official: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, "tester", "install.binary.sh"), []byte("echo BINARY\n"), 0o600); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	cases := []struct {
		name          string
		methods       map[string]string
		wantContains  string
		wantEnvMethod string
	}{
		{"no_override_uses_default", nil, "echo OFFICIAL", "official"},
		{"override_to_binary", map[string]string{"tester": "binary"}, "echo BINARY", "binary"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, err := generatePluginInstalls(
				plugins, enabled, os.DirFS(pluginsDir), nil,
				map[string]config.PluginVersionOverride{},
				tc.methods,
				&bytes.Buffer{},
				shellEnv{rcFileAbs: "/home/${USERNAME}/.bashrc", rcSyntax: "posix", loginShell: "bash"},
			)
			if err != nil {
				t.Fatalf("generatePluginInstalls: %v", err)
			}
			if !strings.Contains(out, tc.wantContains) {
				t.Errorf("output missing %q:\n%s", tc.wantContains, out)
			}
			wantEnv := `COCOON_INSTALL_METHOD="` + tc.wantEnvMethod + `"`
			if !strings.Contains(out, wantEnv) {
				t.Errorf("output missing %q:\n%s", wantEnv, out)
			}
		})
	}
}

// TestGeneratePluginInstalls_LegacyPluginNoMethodEnv pins backward
// compatibility: a plugin without [install.methods] does not get a
// COCOON_INSTALL_METHOD env var (keeps existing-plugin golden output
// byte-identical).
func TestGeneratePluginInstalls_LegacyPluginNoMethodEnv(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: plugin.Metadata{Name: "legacy", URL: "https://example.com/x"},
		Install:  plugin.Install{RequiresRoot: false},
	}
	plugins := map[string]*plugin.Plugin{"legacy": p}
	enabled := []string{"legacy"}
	pluginsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pluginsDir, "legacy"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, "legacy", "install.sh"), []byte("echo LEGACY\n"), 0o600); err != nil {
		t.Fatalf("write install.sh: %v", err)
	}

	out, err := generatePluginInstalls(
		plugins, enabled, os.DirFS(pluginsDir), nil,
		map[string]config.PluginVersionOverride{},
		nil,
		&bytes.Buffer{},
		shellEnv{rcFileAbs: "/home/${USERNAME}/.bashrc", rcSyntax: "posix", loginShell: "bash"},
	)
	if err != nil {
		t.Fatalf("generatePluginInstalls: %v", err)
	}
	if !strings.Contains(out, "echo LEGACY") {
		t.Errorf("output missing legacy install body:\n%s", out)
	}
	if strings.Contains(out, "COCOON_INSTALL_METHOD") {
		t.Errorf("legacy plugin must not carry COCOON_INSTALL_METHOD env:\n%s", out)
	}
}

// TestMethodVerifiesByChecksum pins which install method categories consume the
// $CHECKSUM_AMD64 / $CHECKSUM_ARM64 env pair: only binary and archive download a
// discrete asset and run sha256sum -c. installer and apt verify by other means;
// an empty (legacy single-install.sh) or custom category defaults to
// checksum-capable so existing plugins keep their warning + build args.
func TestMethodVerifiesByChecksum(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		method string
		want   bool
	}{
		{"binary", "binary", true},
		{"archive", "archive", true},
		{"installer", "installer", false},
		{"apt", "apt", false},
		{"empty_legacy", "", true},
		{"custom_category", "custom", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := methodVerifiesByChecksum(tc.method); got != tc.want {
				t.Errorf("methodVerifiesByChecksum(%q) = %v, want %v", tc.method, got, tc.want)
			}
		})
	}
}

// TestBuildInstallEnvPairs_VerifyGatesChecksum pins that CHECKSUM_AMD64 /
// CHECKSUM_ARM64 are injected only for checksum-verified version_capable plugins
// whose install method consumes them (binary / archive). A pgp plugin and an
// installer / apt method plugin get PIN but no CHECKSUM_*; a non-version_capable
// plugin gets neither.
func TestBuildInstallEnvPairs_VerifyGatesChecksum(t *testing.T) {
	t.Parallel()
	csum := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	override := config.PluginVersionOverride{Pin: "1.2.3", ChecksumAmd64: &csum, ChecksumArm64: &csum}
	sh := shellEnv{rcFileAbs: "/home/${USERNAME}/.bashrc", rcSyntax: "posix", loginShell: "bash"}
	cases := []struct {
		name           string
		versionCapable bool
		checksumVerify bool
		method         string
		wantPIN        bool
		wantChecksum   bool
	}{
		{"binary_gets_pin_and_checksum", true, true, "binary", true, true},
		{"archive_gets_pin_and_checksum", true, true, "archive", true, true},
		{"legacy_empty_method_gets_pin_and_checksum", true, true, "", true, true},
		{"pgp_plugin_gets_pin_only", true, false, "binary", true, false},
		{"installer_method_gets_pin_only", true, true, "installer", true, false},
		{"apt_method_gets_pin_only", true, true, "apt", true, false},
		{"not_version_capable_gets_neither", false, true, "binary", false, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pairs := buildInstallEnvPairs(tc.versionCapable, tc.checksumVerify, true, override, nil, nil, tc.method, sh)
			joined := strings.Join(pairs, " ")
			if gotPIN := strings.Contains(joined, `PIN="1.2.3"`); gotPIN != tc.wantPIN {
				t.Errorf("PIN present = %v, want %v\n%s", gotPIN, tc.wantPIN, joined)
			}
			gotChecksum := strings.Contains(joined, "CHECKSUM_AMD64=") || strings.Contains(joined, "CHECKSUM_ARM64=")
			if gotChecksum != tc.wantChecksum {
				t.Errorf("CHECKSUM_* present = %v, want %v\n%s", gotChecksum, tc.wantChecksum, joined)
			}
		})
	}
}

// TestValidateManualChecksums pins the [plugins.options] manual-checksum gate:
// a hand-typed checksum is rejected for an installer / apt method plugin (whose
// install script ignores $CHECKSUM_*), for a pgp plugin (the checksum vocabulary
// does not apply), and for an auto-resolvable plugin (cocoon lock owns the
// checksum), but allowed for a binary / archive plugin whose upstream publishes
// none (or that declares no source at all).
func TestValidateManualChecksums(t *testing.T) {
	t.Parallel()
	csum := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	mkPlugin := func(verify string, src *plugin.VersionSource, method string) *plugin.Plugin {
		inst := plugin.Install{}
		if method != "" {
			inst = plugin.Install{
				DefaultMethod: method,
				Methods:       map[string]plugin.InstallMethod{method: {Description: "x"}},
			}
		}
		return &plugin.Plugin{
			Metadata: plugin.Metadata{Name: "x", URL: "https://example.com/x"},
			Install:  inst,
			Version:  plugin.Version{VersionCapable: true, Verify: verify, Source: src},
		}
	}
	sidecar := &plugin.VersionSource{Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumSidecar}}
	noneSrc := &plugin.VersionSource{Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone}}
	cases := []struct {
		name    string
		plug    *plugin.Plugin
		wantErr bool
	}{
		{"installer_method_rejected", mkPlugin(plugin.VerifyChecksum, noneSrc, "installer"), true},
		{"apt_method_rejected", mkPlugin(plugin.VerifyChecksum, noneSrc, "apt"), true},
		{"pgp_rejected", mkPlugin(plugin.VerifyPGP, nil, ""), true},
		{"auto_resolvable_rejected", mkPlugin(plugin.VerifyChecksum, sidecar, ""), true},
		{"none_type_allowed", mkPlugin(plugin.VerifyChecksum, noneSrc, ""), false},
		{"binary_none_type_allowed", mkPlugin(plugin.VerifyChecksum, noneSrc, "binary"), false},
		{"no_source_allowed", mkPlugin(plugin.VerifyChecksum, nil, ""), false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateManualChecksums(
				map[string]*plugin.Plugin{"x": tc.plug},
				map[string]config.PluginVersionOverride{"x": {Pin: "2.0.0", ChecksumAmd64: &csum}},
				nil,
			)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidVersionOverride) {
					t.Fatalf("err = %v, want errors.Is(.., ErrInvalidVersionOverride)", err)
				}
				return
			}
			if err != nil {
				t.Errorf("manual checksum must be allowed here, got: %v", err)
			}
		})
	}
}

// TestValidateVersionOverrides_PinWithoutChecksumWarning pins the
// missing-checksum WARNING contract: it is suppressed for a pgp plugin (fully
// verified in-script) and for an installer / apt method plugin (whose install
// script ignores $CHECKSUM_*), and its wording tracks whether the source can
// auto-resolve a checksum — an auto-resolvable plugin still verifies and points
// at `cocoon lock`, while a none/no-source plugin warns that the install runs
// WITHOUT verification and points at [plugins.options].
func TestValidateVersionOverrides_PinWithoutChecksumWarning(t *testing.T) {
	t.Parallel()
	sidecar := &plugin.VersionSource{Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumSidecar}} //nolint:exhaustruct // only the kind matters here
	noneSrc := &plugin.VersionSource{Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone}}    //nolint:exhaustruct // only the kind matters here
	cases := []struct {
		name         string
		verify       string
		src          *plugin.VersionSource
		method       string
		wantWarn     bool
		wantContains string
	}{
		{"auto_resolvable_points_at_lock", plugin.VerifyChecksum, sidecar, "binary", true, "cocoon lock"},
		{"none_type_warns_unverified", plugin.VerifyChecksum, noneSrc, "binary", true, "WITHOUT verification"},
		{"no_source_warns_unverified", plugin.VerifyChecksum, nil, "archive", true, "WITHOUT verification"},
		{"installer_method_silent", plugin.VerifyChecksum, noneSrc, "installer", false, ""},
		{"apt_method_silent", plugin.VerifyChecksum, noneSrc, "apt", false, ""},
		{"pgp_plugin_silent", plugin.VerifyPGP, nil, "binary", false, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plugins := map[string]*plugin.Plugin{
				"p": {
					Metadata: plugin.Metadata{Name: "p", URL: "https://example.com/x"},
					Install: plugin.Install{
						DefaultMethod: tc.method,
						Methods:       map[string]plugin.InstallMethod{tc.method: {Description: "x"}},
					},
					Version: plugin.Version{VersionCapable: true, Verify: tc.verify, Source: tc.src},
				},
			}
			var warnings bytes.Buffer
			err := validateVersionOverrides(
				plugins,
				map[string]config.PluginVersionOverride{"p": {Pin: "1.0.0"}},
				nil,
				&warnings,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotWarn := strings.Contains(warnings.String(), "WARNING"); gotWarn != tc.wantWarn {
				t.Errorf("warning emitted = %v, want %v\n%s", gotWarn, tc.wantWarn, warnings.String())
			}
			if tc.wantContains != "" && !strings.Contains(warnings.String(), tc.wantContains) {
				t.Errorf("warning %q does not contain %q", warnings.String(), tc.wantContains)
			}
		})
	}
}

// TestGeneratePluginInstalls_UnknownMethodFails pins that a workspace
// override naming an undeclared method surfaces ErrUnknownMethod (not
// a silently dropped plugin).
func TestGeneratePluginInstalls_UnknownMethodFails(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: plugin.Metadata{Name: "tester", URL: "https://example.com/x"},
		Install: plugin.Install{
			DefaultMethod: "official",
			Methods: map[string]plugin.InstallMethod{
				"official": {Description: "Official"},
			},
		},
	}
	plugins := map[string]*plugin.Plugin{"tester": p}
	pluginsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pluginsDir, "tester"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, "tester", "install.official.sh"), []byte("echo OFFICIAL\n"), 0o600); err != nil {
		t.Fatalf("write official: %v", err)
	}

	_, err := generatePluginInstalls(
		plugins, []string{"tester"}, os.DirFS(pluginsDir), nil,
		map[string]config.PluginVersionOverride{},
		map[string]string{"tester": "ghost"},
		&bytes.Buffer{},
		shellEnv{rcFileAbs: "/home/${USERNAME}/.bashrc", rcSyntax: "posix", loginShell: "bash"},
	)
	if !errors.Is(err, plugin.ErrUnknownMethod) {
		t.Fatalf("err = %v, want errors.Is(.., plugin.ErrUnknownMethod)", err)
	}
}

// TestBuildInstallEnvPairs_ExtraVersions pins that [install.extra_versions]
// keys surface in the env prefix block: defaults are used when no
// workspace override exists, the override wins when both are present,
// and the env-pair order is stable (sorted by key) so the generated
// Dockerfile does not drift.
func TestBuildInstallEnvPairs_ExtraVersions(t *testing.T) {
	t.Parallel()
	sh := shellEnv{rcFileAbs: "/home/${USERNAME}/.bashrc", rcSyntax: "posix", loginShell: "bash"}
	extras := map[string]plugin.ExtraVersionSpec{
		"api_level":   {Env: "ANDROID_SDK_API_LEVEL", Default: "35"},
		"build_tools": {Env: "ANDROID_SDK_BUILD_TOOLS", Default: "35.0.0"},
	}
	t.Run("defaults_when_no_override", func(t *testing.T) {
		t.Parallel()
		pairs := buildInstallEnvPairs(false, false, false, config.PluginVersionOverride{}, nil, extras, "", sh)
		joined := strings.Join(pairs, " ")
		if !strings.Contains(joined, `ANDROID_SDK_API_LEVEL="35"`) {
			t.Errorf("missing default api_level env: %s", joined)
		}
		if !strings.Contains(joined, `ANDROID_SDK_BUILD_TOOLS="35.0.0"`) {
			t.Errorf("missing default build_tools env: %s", joined)
		}
	})
	t.Run("override_wins_over_default", func(t *testing.T) {
		t.Parallel()
		override := config.PluginVersionOverride{
			Pin:   "14742923",
			Extra: map[string]string{"api_level": "36"},
		}
		pairs := buildInstallEnvPairs(true, true, true, override, nil, extras, "", sh)
		joined := strings.Join(pairs, " ")
		if !strings.Contains(joined, `ANDROID_SDK_API_LEVEL="36"`) {
			t.Errorf("override did not take effect for api_level: %s", joined)
		}
		// build_tools has no override; default must still appear.
		if !strings.Contains(joined, `ANDROID_SDK_BUILD_TOOLS="35.0.0"`) {
			t.Errorf("build_tools fell out of env: %s", joined)
		}
	})
	t.Run("sort_order_stable", func(t *testing.T) {
		t.Parallel()
		pairs := buildInstallEnvPairs(false, false, false, config.PluginVersionOverride{}, nil, extras, "", sh)
		apiIdx, buildIdx := -1, -1
		for i, p := range pairs {
			if strings.HasPrefix(p, "ANDROID_SDK_API_LEVEL=") {
				apiIdx = i
			}
			if strings.HasPrefix(p, "ANDROID_SDK_BUILD_TOOLS=") {
				buildIdx = i
			}
		}
		if apiIdx < 0 || buildIdx < 0 {
			t.Fatalf("expected both extra envs to appear; pairs=%v", pairs)
		}
		if apiIdx >= buildIdx {
			t.Errorf("expected api_level (sorted first) before build_tools; api=%d build=%d", apiIdx, buildIdx)
		}
	})
}

// TestValidateVersionOverrides_UnknownExtraKey pins that
// [plugins.options].<id> setting a key the plugin does not declare under
// [install.extra_versions] is rejected with ErrUnknownExtraVersion. This is
// the typo-detection path.
func TestValidateVersionOverrides_UnknownExtraKey(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: plugin.Metadata{Name: "android-sdk", URL: "https://example.com/x"},
		Install: plugin.Install{
			DefaultMethod: "archive",
			Methods:       map[string]plugin.InstallMethod{"archive": {Description: "x"}},
			ExtraVersions: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "ANDROID_SDK_API_LEVEL", Default: "35"},
			},
		},
		Version: plugin.Version{VersionCapable: true},
	}
	plugins := map[string]*plugin.Plugin{"android-sdk": p}
	overrides := map[string]config.PluginVersionOverride{
		"android-sdk": {
			Pin:   "14742923",
			Extra: map[string]string{"api_lvl": "35"}, // typo!
		},
	}
	err := validateVersionOverrides(plugins, overrides, nil, &bytes.Buffer{})
	if !errors.Is(err, ErrUnknownExtraVersion) {
		t.Fatalf("err = %v, want errors.Is(.., ErrUnknownExtraVersion)", err)
	}
	if !strings.Contains(err.Error(), "api_lvl") {
		t.Errorf("error message should name the unknown key: %v", err)
	}
}

// TestValidateVersionOverrides_DeclaredExtraKeyOK pins the happy path:
// a workspace override whose Extra keys are all declared by the plugin
// passes validation without error.
func TestValidateVersionOverrides_DeclaredExtraKeyOK(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: plugin.Metadata{Name: "android-sdk", URL: "https://example.com/x"},
		Install: plugin.Install{
			DefaultMethod: "archive",
			Methods:       map[string]plugin.InstallMethod{"archive": {Description: "x"}},
			ExtraVersions: map[string]plugin.ExtraVersionSpec{
				"api_level":   {Env: "ANDROID_SDK_API_LEVEL", Default: "35"},
				"build_tools": {Env: "ANDROID_SDK_BUILD_TOOLS", Default: "35.0.0"},
			},
		},
		Version: plugin.Version{VersionCapable: true},
	}
	plugins := map[string]*plugin.Plugin{"android-sdk": p}
	overrides := map[string]config.PluginVersionOverride{
		"android-sdk": {
			Pin:   "14742923",
			Extra: map[string]string{"api_level": "36", "build_tools": "36.0.0"},
		},
	}
	if err := validateVersionOverrides(plugins, overrides, nil, &bytes.Buffer{}); err != nil {
		t.Fatalf("validateVersionOverrides: %v", err)
	}
}
