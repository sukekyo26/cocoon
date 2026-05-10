//nolint:testpackage // exercises unexported helpers (applyFlags, applyDefaults, validators, ...).
package initcli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// loadPluginsForTest loads the embedded plugin catalog so tests can drive
// applyFlags / applyDefaults / validatePluginConflicts against the same
// data the production runInit path would see. t.Fatal on failure because
// catalog loading is a pure compile-time / embed-time guarantee.
func loadPluginsForTest(t *testing.T) map[string]*plugin.Plugin {
	t.Helper()
	plugins, err := loadEmbeddedPlugins()
	if err != nil {
		t.Fatalf("loadEmbeddedPlugins: %v", err)
	}
	return plugins
}

// pinEnglish forces the i18n catalog to English so assertions stay
// stable on hosts whose LANG starts with "ja".
func pinEnglish(t *testing.T) {
	t.Helper()
	for _, k := range []string{"WORKSPACE_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(k, "")
	}
	t.Setenv("WORKSPACE_LANG", "en")
}

// ---------------------------------------------------------------------
// makeStrictValidator: human messages, no regex leak, accepts valid input.
// ---------------------------------------------------------------------

func TestMakeStrictValidator_Empty(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		lang    i18n.Lang
		wantSub string
	}{
		{"en", i18n.LangEN, "please enter"},
		{"ja", i18n.LangJA, "必須"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cat := i18n.New(tc.lang)
			err := makeStrictValidator(rxServiceName, "init_err_service_name_fmt", cat)("")
			if err == nil {
				t.Fatal("expected error for empty input")
			}
			msg := err.Error()
			if strings.Contains(msg, "^[") || strings.Contains(msg, "*$") {
				t.Errorf("must not leak regex: %q", msg)
			}
			if !strings.Contains(strings.ToLower(msg), strings.ToLower(tc.wantSub)) {
				t.Errorf("msg %q missing %q", msg, tc.wantSub)
			}
		})
	}
}

func TestMakeStrictValidator_BadCharsServiceName(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	for _, in := range []string{"Bad-Name", "_underscore", "1starts-with-digit", "has space", "has.dot", "has/slash"} {
		err := makeStrictValidator(rxServiceName, "init_err_service_name_fmt", cat)(in)
		if err == nil {
			t.Errorf("expected error for service-name %q, got nil", in)
			continue
		}
		if strings.Contains(err.Error(), "^[") {
			t.Errorf("service-name %q error leaks regex: %q", in, err.Error())
		}
	}
}

func TestMakeStrictValidator_BadCharsUsername(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	for _, in := range []string{"BadUser", "1user", "user space", "user.dot"} {
		err := makeStrictValidator(rxUsername, "init_err_username_fmt", cat)(in)
		if err == nil {
			t.Errorf("expected error for username %q, got nil", in)
		}
	}
}

func TestMakeStrictValidator_AcceptsValid(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	for _, in := range []string{"a", "myapp", "my-api", "my_api", "api123", "x-1_2-3"} {
		if err := makeStrictValidator(rxServiceName, "init_err_service_name_fmt", cat)(in); err != nil {
			t.Errorf("strict svc validator rejected %q: %v", in, err)
		}
	}
	for _, in := range []string{"dev", "_dev", "user-1", "u_2", "_"} {
		if err := makeStrictValidator(rxUsername, "init_err_username_fmt", cat)(in); err != nil {
			t.Errorf("strict username validator rejected %q: %v", in, err)
		}
	}
}

// Pin: username allows leading underscore, service-name does not.
func TestRegex_LeadingUnderscoreAsymmetry(t *testing.T) {
	t.Parallel()
	if rxServiceName.MatchString("_underscore") {
		t.Error("service-name must reject leading _")
	}
	if !rxUsername.MatchString("_underscore") {
		t.Error("username must accept leading _")
	}
}

// ---------------------------------------------------------------------
// applyFlags: each flag validated, *Set bookkeeping correct.
// ---------------------------------------------------------------------

func TestApplyFlags_AllValid(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	flags := initFlags{
		AutoYes: true, ServiceName: "myapp", Username: "dev",
		OS: "ubuntu", OSVersion: "24.04", MountRoot: "..",
		Devcontainer: false, NoDevcontainer: true,
		AptCategories: "text-editors,build", Force: false,
	}
	ans, err := applyFlags(&flags, plugins)
	if err != nil {
		t.Fatalf("applyFlags: %v", err)
	}
	if ans.ServiceName != "myapp" {
		t.Errorf("ServiceName = %q", ans.ServiceName)
	}
	if ans.Username != "dev" {
		t.Errorf("Username = %q", ans.Username)
	}
	if ans.OS != "ubuntu" || !ans.OSSet {
		t.Errorf("OS = %q OSSet=%v", ans.OS, ans.OSSet)
	}
	if ans.OSVersion != "24.04" || !ans.OSVersionSet {
		t.Errorf("OSVersion = %q OSVersionSet=%v", ans.OSVersion, ans.OSVersionSet)
	}
	if ans.MountRoot != ".." || !ans.MountRootSet {
		t.Errorf("MountRoot = %q MountRootSet=%v", ans.MountRoot, ans.MountRootSet)
	}
	if ans.Devcontainer || !ans.DevcontainerSet {
		t.Errorf("Devcontainer=%v DevcontainerSet=%v", ans.Devcontainer, ans.DevcontainerSet)
	}
	if !ans.AptSet || len(ans.AptCategories) != 2 {
		t.Errorf("AptCategories=%v AptSet=%v", ans.AptCategories, ans.AptSet)
	}
}

func TestApplyFlags_UnsetLeavesZero(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	ans, err := applyFlags(&initFlags{}, plugins)
	if err != nil {
		t.Fatalf("applyFlags: %v", err)
	}
	if ans.ServiceName != "" || ans.Username != "" || ans.OSSet || ans.OSVersionSet ||
		ans.MountRootSet || ans.DevcontainerSet || ans.AptSet {
		t.Errorf("expected fully zero answers, got %+v", ans)
	}
}

func TestApplyFlags_InvalidServiceName(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	for _, bad := range []string{"BAD", "with space", "1leading", "_under"} {
		_, err := applyFlags(&initFlags{ServiceName: bad}, plugins)
		if !errors.Is(err, ErrUsage) {
			t.Errorf("%q → expected ErrUsage, got %v", bad, err)
		}
	}
}

func TestApplyFlags_InvalidUsername(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	for _, bad := range []string{"BAD", "with space", "1leading"} {
		_, err := applyFlags(&initFlags{Username: bad}, plugins)
		if !errors.Is(err, ErrUsage) {
			t.Errorf("%q → expected ErrUsage, got %v", bad, err)
		}
	}
}

func TestApplyFlags_InvalidOS(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	_, err := applyFlags(&initFlags{OS: "alpine"}, plugins)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("expected ErrUsage for unknown --os, got %v", err)
	}
}

func TestApplyFlags_OSVersionWithoutOS(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	_, err := applyFlags(&initFlags{OSVersion: "24.04"}, plugins)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("--os-version without --os should be ErrUsage, got %v", err)
	}
}

func TestApplyFlags_OSVersionMismatch(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	_, err := applyFlags(&initFlags{OS: "debian", OSVersion: "24.04"}, plugins)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("ubuntu version on debian should be ErrUsage, got %v", err)
	}
}

func TestApplyFlags_OSVersionValidPair(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	ans, err := applyFlags(&initFlags{OS: "debian", OSVersion: "13"}, plugins)
	if err != nil {
		t.Fatalf("applyFlags: %v", err)
	}
	if ans.OSVersion != "13" || !ans.OSVersionSet {
		t.Errorf("got %+v", ans)
	}
}

func TestApplyFlags_InvalidMountRoot(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	_, err := applyFlags(&initFlags{MountRoot: "/abs"}, plugins)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("expected ErrUsage for /abs mount-root, got %v", err)
	}
}

func TestApplyFlags_DevcontainerExclusivity(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	// applyFlags itself does not detect this — runInit does. Confirm that
	// each flag in isolation produces the matching boolean.
	ans, err := applyFlags(&initFlags{Devcontainer: true}, plugins)
	if err != nil || !ans.Devcontainer || !ans.DevcontainerSet {
		t.Errorf("--devcontainer should set true: %v %+v", err, ans)
	}
	ans, err = applyFlags(&initFlags{NoDevcontainer: true}, plugins)
	if err != nil || ans.Devcontainer || !ans.DevcontainerSet {
		t.Errorf("--no-devcontainer should set false: %v %+v", err, ans)
	}
}

func TestApplyFlags_UnknownAptCategory(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	_, err := applyFlags(&initFlags{AptCategories: "text-editors,not-a-real-category"}, plugins)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("expected ErrUsage for unknown apt category, got %v", err)
	}
}

func TestApplyFlags_AptCategoriesWhitespace(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	ans, err := applyFlags(&initFlags{AptCategories: " text-editors , build , "}, plugins)
	if err != nil {
		t.Fatalf("applyFlags: %v", err)
	}
	if len(ans.AptCategories) != 2 || ans.AptCategories[0] != "text-editors" || ans.AptCategories[1] != "build" {
		t.Errorf("got %v", ans.AptCategories)
	}
}

// ---------------------------------------------------------------------
// applyDefaults: required-with-yes; defaults respect *Set.
// ---------------------------------------------------------------------

func TestApplyDefaults_RequiresServiceName(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	_, err := applyDefaults(initAnswers{Username: "dev"}, plugins)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("--yes without service_name should be ErrUsage, got %v", err)
	}
}

func TestApplyDefaults_RequiresUsername(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	_, err := applyDefaults(initAnswers{ServiceName: "x"}, plugins)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("--yes without username should be ErrUsage, got %v", err)
	}
}

func TestApplyDefaults_FillsMissingDefaults(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	ans, err := applyDefaults(initAnswers{ServiceName: "svc", Username: "dev"}, plugins)
	if err != nil {
		t.Fatal(err)
	}
	if ans.OS != "ubuntu" || !ans.OSSet {
		t.Errorf("OS default = %q OSSet=%v", ans.OS, ans.OSSet)
	}
	if ans.OSVersion != "26.04" || !ans.OSVersionSet {
		t.Errorf("OSVersion default = %q", ans.OSVersion)
	}
	if ans.MountRoot != "." || !ans.MountRootSet {
		t.Errorf("MountRoot default = %q", ans.MountRoot)
	}
	if !ans.Devcontainer || !ans.DevcontainerSet {
		t.Errorf("Devcontainer default = %v", ans.Devcontainer)
	}
	if !ans.AptSet || len(ans.AptCategories) == 0 {
		t.Errorf("AptCategories default empty: %v", ans.AptCategories)
	}
	if ans.Shell != "bash" || !ans.ShellSet {
		t.Errorf("Shell default = %q ShellSet=%v", ans.Shell, ans.ShellSet)
	}
}

func TestApplyDefaults_PreservesExplicitSettings(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	in := initAnswers{
		ServiceName: "svc",
		Username:    "dev",
		OS:          "debian", OSSet: true,
		OSVersion: "13", OSVersionSet: true,
		MountRoot: "..", MountRootSet: true,
		Devcontainer: false, DevcontainerSet: true,
		AptCategories: []string{"text-editors"}, AptSet: true,
	}
	ans, err := applyDefaults(in, plugins)
	if err != nil {
		t.Fatal(err)
	}
	if ans.OS != "debian" || ans.OSVersion != "13" || ans.MountRoot != ".." ||
		ans.Devcontainer || len(ans.AptCategories) != 1 {
		t.Errorf("explicit values not preserved: %+v", ans)
	}
}

// ---------------------------------------------------------------------
// defaultOSVersion: returns the first (newest) entry per OS.
// ---------------------------------------------------------------------

func TestDefaultOSVersion(t *testing.T) {
	t.Parallel()
	if got := defaultOSVersion("ubuntu"); got != "26.04" {
		t.Errorf("ubuntu default = %q, want 26.04", got)
	}
	if got := defaultOSVersion("debian"); got != "13" {
		t.Errorf("debian default = %q, want 13", got)
	}
	if got := defaultOSVersion("alpine"); got != "" {
		t.Errorf("unknown OS default should be \"\", got %q", got)
	}
}

// ---------------------------------------------------------------------
// versionMatchesOS: catches stale version after OS change in form.
// ---------------------------------------------------------------------

func TestVersionMatchesOS(t *testing.T) {
	t.Parallel()
	if !versionMatchesOS("ubuntu", "24.04") {
		t.Error("ubuntu 24.04 should match")
	}
	if versionMatchesOS("ubuntu", "13") {
		t.Error("ubuntu 13 should NOT match")
	}
	if versionMatchesOS("debian", "24.04") {
		t.Error("debian 24.04 should NOT match")
	}
	if versionMatchesOS("alpine", "any") {
		t.Error("alpine should not match anything")
	}
}

// ---------------------------------------------------------------------
// parseAptCategories: validation and whitespace handling.
// ---------------------------------------------------------------------

func TestParseAptCategories(t *testing.T) {
	t.Parallel()
	out, err := parseAptCategories("text-editors,build")
	if err != nil || len(out) != 2 {
		t.Errorf("got %v %v", out, err)
	}
	out, err = parseAptCategories("")
	if err != nil || len(out) != 0 {
		t.Errorf("empty string → empty list, got %v %v", out, err)
	}
	out, err = parseAptCategories(",,,")
	if err != nil || len(out) != 0 {
		t.Errorf("only commas → empty list, got %v %v", out, err)
	}
	if _, err := parseAptCategories("not-a-real-cat"); !errors.Is(err, ErrUsage) {
		t.Errorf("unknown category should be ErrUsage, got %v", err)
	}
}

// ---------------------------------------------------------------------
// renderWorkspaceToml: output shape pinned for both apt-empty and apt-set.
// ---------------------------------------------------------------------

func TestRenderWorkspaceToml_NoPackages(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", OS: "ubuntu", OSVersion: "24.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true, Packages: nil,
	}, cat)
	for _, want := range []string{
		`mount_root = "."`,
		`devcontainer = true`,
		`service_name = "svc"`,
		`username = "dev"`,
		`os = "ubuntu"`,
		`os_version = "24.04"`,
		"[container.shell]\ndefault = \"bash\"",
		`enable = []`,
		`packages = []`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestRenderWorkspaceToml_WithPackages(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", OS: "debian", OSVersion: "13",
		Shell: "zsh", MountRoot: "..", Devcontainer: false,
		Packages: []string{"vim", "tmux"},
	}, cat)
	for _, want := range []string{
		`mount_root = ".."`,
		`devcontainer = false`,
		`os = "debian"`,
		`os_version = "13"`,
		"[container.shell]\ndefault = \"zsh\"",
		"packages = [\n    \"vim\",\n    \"tmux\",\n]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestRenderWorkspaceToml_WithPlugins(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", OS: "ubuntu", OSVersion: "24.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
		Plugins: []string{"go", "uv", "github-cli"},
	}, cat)
	want := "[plugins]\nenable = [\n    \"go\",\n    \"uv\",\n    \"github-cli\",\n]"
	if !strings.Contains(got, want) {
		t.Errorf("output missing %q\n--- got ---\n%s", want, got)
	}
}

func TestRenderWorkspaceToml_FishShell(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", OS: "ubuntu", OSVersion: "24.04",
		Shell: "fish", MountRoot: ".", Devcontainer: true,
	}, cat)
	want := "[container.shell]\ndefault = \"fish\""
	if !strings.Contains(got, want) {
		t.Errorf("output missing %q\n--- got ---\n%s", want, got)
	}
}

func TestRenderWorkspaceToml_WithAliases(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", OS: "ubuntu", OSVersion: "24.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
		Aliases: map[string]string{"gs": "git status", "ll": "ls -lah"},
	}, cat)
	// Inline-table format with sorted keys (gs before ll alphabetically).
	want := `aliases = { gs = "git status", ll = "ls -lah" }`
	if !strings.Contains(got, want) {
		t.Errorf("output missing %q\n--- got ---\n%s", want, got)
	}
}

func TestRenderWorkspaceToml_NoAliases_OmitsLine(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", OS: "ubuntu", OSVersion: "24.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
		Aliases: nil,
	}, cat)
	if strings.Contains(got, "aliases =") {
		t.Errorf("expected no aliases line when Aliases is nil, got:\n%s", got)
	}
}

// TestRenderWorkspaceToml_LocalizedComments_EN pins the English section
// header line. The full block is allowed to evolve, but this prefix is the
// load-bearing self-documentation for users opening workspace.toml.
func TestRenderWorkspaceToml_LocalizedComments_EN(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", OS: "ubuntu", OSVersion: "24.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
	}, cat)
	for _, want := range []string{
		"# workspace.toml — cocoon configuration",
		"# [workspace] — generation-wide knobs.",
		"# [container] — image identity.",
		"# [container.shell] — login shell + per-shell rc injection.",
		"# [plugins] — enable cocoon plugins",
		"# [apt] — extra apt packages",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("EN comment missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestRenderWorkspaceToml_LocalizedComments_JA(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangJA)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", OS: "ubuntu", OSVersion: "24.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
	}, cat)
	for _, want := range []string{
		"# workspace.toml — cocoon 設定",
		"# [workspace] — 生成全体の挙動。",
		"# [container] — イメージの素性。",
		"# [container.shell] — ログインシェル",
		"# [plugins] — cocoon プラグインの有効化",
		"# [apt] — cocoon の最小ベース",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("JA comment missing %q\n--- got ---\n%s", want, got)
		}
	}
}

// TestRenderWorkspaceToml_ContainerShellEnvCaveats pins the EDITOR/PAGER
// caveats next to [container.shell]. EDITOR/PAGER are intentionally NOT
// init prompts because the values silently break when the prerequisite
// apt category or VS Code is missing — so the comment block must keep
// surfacing those prerequisites in both locales.
func TestRenderWorkspaceToml_ContainerShellEnvCaveats(t *testing.T) {
	t.Parallel()
	for _, lang := range []i18n.Lang{i18n.LangEN, i18n.LangJA} {
		cat := i18n.New(lang)
		got := renderWorkspaceToml(containerSpec{
			ServiceName: "svc", Username: "dev", OS: "ubuntu", OSVersion: "24.04",
			Shell: "bash", MountRoot: ".", Devcontainer: true,
		}, cat)
		for _, want := range []string{"text-editors", "VS Code", "utilities"} {
			if !strings.Contains(got, want) {
				t.Errorf("[%s] caveat keyword %q missing\n--- got ---\n%s", lang, want, got)
			}
		}
	}
}

// ---------------------------------------------------------------------
// runInit (--yes path) end-to-end against a tempdir workspace.toml.
// ---------------------------------------------------------------------

// runInit tests cannot use t.Parallel() because t.Chdir mutates process
// cwd; running them in parallel would race on os.Getwd inside runInit.

//nolint:paralleltest // t.Chdir
func TestRunInit_YesWritesWorkspaceToml(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)

	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "myapp", "--username", "dev",
		"--os", "ubuntu", "--os-version", "24.04",
		"--mount-root", "..", "--no-devcontainer",
		"--apt-categories", "text-editors",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --yes: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(work, "workspace.toml"))
	if err != nil {
		t.Fatalf("read workspace.toml: %v", err)
	}
	for _, want := range []string{
		`service_name = "myapp"`, `username = "dev"`,
		`os = "ubuntu"`, `os_version = "24.04"`,
		`mount_root = ".."`, `devcontainer = false`,
		`"vim"`, `"nano"`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("workspace.toml missing %q\n--- got ---\n%s", want, body)
		}
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_YesRejectsMissingServiceName(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)

	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{"--yes", "--username", "dev"})
	err := cmd.Execute()
	if !errors.Is(err, ErrUsage) {
		t.Errorf("expected ErrUsage, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(work, "workspace.toml")); statErr == nil {
		t.Error("workspace.toml should NOT have been written when --yes lacks --service-name")
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_YesRejectsMissingUsername(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)

	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{"--yes", "--service-name", "myapp"})
	err := cmd.Execute()
	if !errors.Is(err, ErrUsage) {
		t.Errorf("expected ErrUsage, got %v", err)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_RefusesOverwriteWithoutForce(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte("# pre-existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{"--yes", "--service-name", "x", "--username", "y"})
	if err := cmd.Execute(); !errors.Is(err, ErrUsage) {
		t.Errorf("expected ErrUsage for existing file, got %v", err)
	}
	body, readErr := os.ReadFile(filepath.Join(work, "workspace.toml"))
	if readErr != nil {
		t.Fatalf("read workspace.toml: %v", readErr)
	}
	if string(body) != "# pre-existing\n" {
		t.Error("existing workspace.toml was overwritten despite --force not being set")
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_ForceOverwrites(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte("# stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--force", "--service-name", "newapp", "--username", "dev",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --force: %v", err)
	}
	body, readErr := os.ReadFile(filepath.Join(work, "workspace.toml"))
	if readErr != nil {
		t.Fatalf("read workspace.toml: %v", readErr)
	}
	if !strings.Contains(string(body), `service_name = "newapp"`) {
		t.Errorf("--force should overwrite, got:\n%s", body)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_DevcontainerConflict(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y",
		"--devcontainer", "--no-devcontainer",
	})
	if err := cmd.Execute(); !errors.Is(err, ErrUsage) {
		t.Errorf("conflicting flags should be ErrUsage, got %v", err)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_CertificatesConflict(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y",
		"--certificates", "--no-certificates",
	})
	if err := cmd.Execute(); !errors.Is(err, ErrUsage) {
		t.Errorf("conflicting --certificates flags should be ErrUsage, got %v", err)
	}
}

// TestRunInit_CertificatesFlag verifies that --certificates emits the
// live [certificates] section and the absence of the flag emits the
// commented template. Both branches sit through the same renderer so
// this protects against regressions where the toggle desyncs from the
// emit path.
//
//nolint:paralleltest // t.Chdir
func TestRunInit_CertificatesFlag(t *testing.T) {
	pinEnglish(t)

	cases := []struct {
		name           string
		extraArgs      []string
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:      "enabled",
			extraArgs: []string{"--certificates"},
			mustContain: []string{
				"\n[certificates]\nenable = true\n",
			},
			mustNotContain: []string{
				// Commented template (init_toml_template_certificates)
				// uses "opt in to" wording; the live section header
				// (init_toml_section_certificates) does not.
				"opt in to TLS certificate auto-bake",
				"# enable = true",
			},
		},
		{
			name:           "default-off",
			extraArgs:      nil,
			mustContain:    []string{"opt in to TLS certificate auto-bake", "# enable = true"},
			mustNotContain: []string{"\n[certificates]\nenable = true\n"},
		},
		{
			name:      "explicit-off",
			extraArgs: []string{"--no-certificates"},
			mustContain: []string{
				"opt in to TLS certificate auto-bake",
				"# enable = true",
			},
			mustNotContain: []string{"\n[certificates]\nenable = true\n"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			work := t.TempDir()
			t.Chdir(work)
			args := append([]string{
				"--yes", "--service-name", "dev", "--username", "dev",
				"--os", "ubuntu", "--os-version", "22.04",
				"--mount-root", ".", "--no-devcontainer",
			}, tc.extraArgs...)
			cmd := NewCommand(io.Discard, io.Discard)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("init: %v", err)
			}
			body, err := os.ReadFile(filepath.Join(work, "workspace.toml"))
			if err != nil {
				t.Fatalf("read workspace.toml: %v", err)
			}
			out := string(body)
			for _, want := range tc.mustContain {
				if !strings.Contains(out, want) {
					t.Errorf("workspace.toml missing %q\n--- got ---\n%s", want, out)
				}
			}
			for _, mustNot := range tc.mustNotContain {
				if strings.Contains(out, mustNot) {
					t.Errorf("workspace.toml must not contain %q\n--- got ---\n%s", mustNot, out)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------
// Plugin selection: --plugins flag, conflicts validation, defaults.
// ---------------------------------------------------------------------

func TestParsePlugins(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)

	out, err := parsePlugins("go,uv,github-cli", plugins)
	if err != nil || len(out) != 3 {
		t.Errorf("got %v %v", out, err)
	}
	out, err = parsePlugins("", plugins)
	if err != nil || len(out) != 0 {
		t.Errorf("empty string → empty list, got %v %v", out, err)
	}
	out, err = parsePlugins(" go , uv , ", plugins)
	if err != nil || len(out) != 2 || out[0] != "go" || out[1] != "uv" {
		t.Errorf("whitespace handling: got %v %v", out, err)
	}
	if _, err := parsePlugins("does-not-exist", plugins); !errors.Is(err, ErrUsage) {
		t.Errorf("unknown plugin should be ErrUsage, got %v", err)
	}
}

func TestValidatePluginConflicts(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)

	// custom-ps1 ↔ starship is the canonical conflict pair shipped in the
	// embedded catalog. Picking both must report a conflict.
	err := validatePluginConflicts(plugins, []string{"custom-ps1", "starship"})
	if !errors.Is(err, ErrUsage) {
		t.Errorf("custom-ps1+starship should be ErrUsage, got %v", err)
	}

	// Either alone is fine.
	if err := validatePluginConflicts(plugins, []string{"custom-ps1"}); err != nil {
		t.Errorf("custom-ps1 alone should be ok, got %v", err)
	}
	if err := validatePluginConflicts(plugins, []string{"starship"}); err != nil {
		t.Errorf("starship alone should be ok, got %v", err)
	}

	// Empty list is trivially ok.
	if err := validatePluginConflicts(plugins, nil); err != nil {
		t.Errorf("nil enabled should be ok, got %v", err)
	}

	// Disjoint plugins do not conflict.
	if err := validatePluginConflicts(plugins, []string{"go", "uv", "github-cli"}); err != nil {
		t.Errorf("disjoint plugins should be ok, got %v", err)
	}
}

func TestDefaultPluginIDs(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)

	got := defaultPluginIDs(plugins)
	// The current catalog ships `docker-cli` as the only default-on plugin.
	// If you add another default-on plugin, update this assertion alongside
	// the catalog change.
	want := []string{"docker-cli"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("default plugin ids: got %v, want %v", got, want)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_PluginsFlagWritesEnable(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)

	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "myapp", "--username", "dev",
		"--plugins", "go,uv,github-cli",
		"--apt-categories", "text-editors",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --yes --plugins: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(work, "workspace.toml"))
	if err != nil {
		t.Fatalf("read workspace.toml: %v", err)
	}
	want := "[plugins]\nenable = [\n    \"go\",\n    \"uv\",\n    \"github-cli\",\n]"
	if !strings.Contains(string(body), want) {
		t.Errorf("workspace.toml missing %q\n--- got ---\n%s", want, body)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_PluginsFlagRejectsUnknown(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)

	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y",
		"--plugins", "does-not-exist",
	})
	if err := cmd.Execute(); !errors.Is(err, ErrUsage) {
		t.Errorf("unknown plugin should be ErrUsage, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(work, "workspace.toml")); statErr == nil {
		t.Error("workspace.toml should NOT have been written when --plugins lists an unknown id")
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_PluginsFlagRejectsConflict(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)

	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y",
		"--plugins", "custom-ps1,starship",
	})
	if err := cmd.Execute(); !errors.Is(err, ErrUsage) {
		t.Errorf("conflicting plugins should be ErrUsage, got %v", err)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_YesDefaultsToDockerCli(t *testing.T) {
	// `--yes` with no --plugins should fall back to defaultPluginIDs(),
	// which currently means just docker-cli (the only `default = true`
	// plugin in the embedded catalog).
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)

	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{"--yes", "--service-name", "x", "--username", "y"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --yes: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(work, "workspace.toml"))
	if err != nil {
		t.Fatalf("read workspace.toml: %v", err)
	}
	want := "[plugins]\nenable = [\n    \"docker-cli\",\n]"
	if !strings.Contains(string(body), want) {
		t.Errorf("workspace.toml missing %q\n--- got ---\n%s", want, body)
	}
}

// ---------------------------------------------------------------------
// Plugin version pins: --plugin-versions flag, parser, render output.
// ---------------------------------------------------------------------

func TestParsePluginVersions(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	enabled := []string{"go", "uv", "starship"}

	out, err := parsePluginVersions("go=1.23.4,uv=0.5.7", plugins, enabled)
	if err != nil {
		t.Fatalf("happy path: %v", err)
	}
	if out["go"] != "1.23.4" || out["uv"] != "0.5.7" || len(out) != 2 {
		t.Errorf("happy path map: %v", out)
	}

	// parsePluginVersions returns a non-nil empty map for whitespace-only
	// or empty input so it doesn't trip golangci-lint's `nilnil` rule. The
	// writer keys behavior on len(...) == 0, so the empty-vs-nil distinction
	// is invisible to callers in practice.
	if out, err := parsePluginVersions("", plugins, enabled); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v", out, err)
	}

	if out, err := parsePluginVersions(" go = 1.23.4 ,  uv=0.5.7 ", plugins, enabled); err != nil ||
		out["go"] != "1.23.4" || out["uv"] != "0.5.7" {
		t.Errorf("whitespace handling: out=%v err=%v", out, err)
	}

	for _, bad := range []string{"go", "go=", "=1.23", "go==1.23"} {
		if _, err := parsePluginVersions(bad, plugins, enabled); !errors.Is(err, ErrUsage) {
			t.Errorf("malformed token %q: expected ErrUsage, got %v", bad, err)
		}
	}

	if _, err := parsePluginVersions("does-not-exist=1.0", plugins, enabled); !errors.Is(err, ErrUsage) {
		t.Errorf("unknown plugin: expected ErrUsage, got %v", err)
	}

	// docker-cli ships in the embedded catalog but is `version_capable=false`.
	if _, err := parsePluginVersions("docker-cli=1.0", plugins, []string{"docker-cli"}); !errors.Is(err, ErrUsage) {
		t.Errorf("non-version-capable: expected ErrUsage, got %v", err)
	}

	// `go` is version_capable but the caller did not list it in --plugins.
	if _, err := parsePluginVersions("go=1.23.4", plugins, []string{"uv"}); !errors.Is(err, ErrUsage) {
		t.Errorf("missing-from-enable: expected ErrUsage, got %v", err)
	}

	if _, err := parsePluginVersions("go=1.23.4,go=1.24.0", plugins, enabled); !errors.Is(err, ErrUsage) {
		t.Errorf("duplicate id: expected ErrUsage, got %v", err)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_PluginVersionsFlagWritesBlock(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)

	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "myapp", "--username", "dev",
		"--plugins", "go,uv,starship",
		"--plugin-versions", "go=1.23.4,starship=1.21.1",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --plugin-versions: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(work, "workspace.toml"))
	if err != nil {
		t.Fatalf("read workspace.toml: %v", err)
	}
	// Sorted by id: go before starship; each block is the canonical
	// FormatPinBlock output (no checksums).
	want := "[plugins.versions.go]\npin = \"1.23.4\"\n\n[plugins.versions.starship]\npin = \"1.21.1\"\n"
	if !strings.Contains(string(body), want) {
		t.Errorf("workspace.toml missing pin blocks\n--- want ---\n%s\n--- got ---\n%s", want, body)
	}
	// The commented-out example template must NOT appear when real pins
	// were emitted — otherwise the user sees both the example and their own
	// block. The template's leading comment is unique to the example.
	if strings.Contains(string(body), "pin specific versions for version_capable plugins") {
		t.Errorf("commented [plugins.versions] template should not coexist with real pins\n--- got ---\n%s", body)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_PluginVersionsRejectsUnknownPlugin(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y",
		"--plugins", "go",
		"--plugin-versions", "does-not-exist=1.0",
	})
	if err := cmd.Execute(); !errors.Is(err, ErrUsage) {
		t.Errorf("expected ErrUsage, got %v", err)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_PluginVersionsRejectsNonVersionCapable(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y",
		"--plugins", "docker-cli",
		"--plugin-versions", "docker-cli=1.0",
	})
	if err := cmd.Execute(); !errors.Is(err, ErrUsage) {
		t.Errorf("expected ErrUsage for non-version-capable docker-cli, got %v", err)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_PluginVersionsRejectsMissingFromEnable(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y",
		"--plugins", "uv",
		"--plugin-versions", "go=1.23.4",
	})
	if err := cmd.Execute(); !errors.Is(err, ErrUsage) {
		t.Errorf("expected ErrUsage when pin is for a non-enabled plugin, got %v", err)
	}
}

// ---------------------------------------------------------------------
// Shell selection: --shell flag, validation, defaults, render output.
// ---------------------------------------------------------------------

func TestApplyFlags_ShellValid(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	for _, sh := range []string{"bash", "zsh", "fish"} {
		ans, err := applyFlags(&initFlags{Shell: sh}, plugins)
		if err != nil {
			t.Errorf("--shell %q: %v", sh, err)
			continue
		}
		if ans.Shell != sh || !ans.ShellSet {
			t.Errorf("--shell %q: ans.Shell=%q ShellSet=%v", sh, ans.Shell, ans.ShellSet)
		}
	}
}

func TestApplyFlags_InvalidShell(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	for _, bad := range []string{"csh", "tcsh", "sh", "BASH"} {
		_, err := applyFlags(&initFlags{Shell: bad}, plugins)
		if !errors.Is(err, ErrUsage) {
			t.Errorf("--shell %q: expected ErrUsage, got %v", bad, err)
		}
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_ShellFlagWritesContainerShell(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)

	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y", "--shell", "fish",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --shell fish: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(work, "workspace.toml"))
	if err != nil {
		t.Fatalf("read workspace.toml: %v", err)
	}
	want := "[container.shell]\ndefault = \"fish\""
	if !strings.Contains(string(body), want) {
		t.Errorf("workspace.toml missing %q\n--- got ---\n%s", want, body)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_ShellFlagRejectsInvalid(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y", "--shell", "csh",
	})
	if err := cmd.Execute(); !errors.Is(err, ErrUsage) {
		t.Errorf("--shell csh should be ErrUsage, got %v", err)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_DefaultsShellToBash(t *testing.T) {
	// `--yes` without --shell should bake the bash default into the
	// generated `[container.shell]` block, mirroring the other defaults
	// that always materialize literally.
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{"--yes", "--service-name", "x", "--username", "y"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --yes: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(work, "workspace.toml"))
	if err != nil {
		t.Fatalf("read workspace.toml: %v", err)
	}
	want := "[container.shell]\ndefault = \"bash\""
	if !strings.Contains(string(body), want) {
		t.Errorf("workspace.toml missing %q\n--- got ---\n%s", want, body)
	}
}

// ---------------------------------------------------------------------
// Alias bundles: --alias-bundles flag, parsing, render output.
// ---------------------------------------------------------------------

func TestParseAliasBundles(t *testing.T) {
	t.Parallel()
	out, err := parseAliasBundles("git,ls,docker")
	if err != nil || len(out) != 3 {
		t.Errorf("got %v %v", out, err)
	}
	out, err = parseAliasBundles("")
	if err != nil || len(out) != 0 {
		t.Errorf("empty string -> empty list, got %v %v", out, err)
	}
	out, err = parseAliasBundles(" git , ls , ")
	if err != nil || len(out) != 2 || out[0] != "git" || out[1] != "ls" {
		t.Errorf("whitespace handling: got %v %v", out, err)
	}
	if _, err := parseAliasBundles("k8s"); !errors.Is(err, ErrUsage) {
		t.Errorf("unknown bundle should be ErrUsage, got %v", err)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_AliasBundlesFlagWritesShellAliases(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y",
		"--alias-bundles", "git,ls",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --alias-bundles: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(work, "workspace.toml"))
	if err != nil {
		t.Fatalf("read workspace.toml: %v", err)
	}
	for _, want := range []string{
		`ga = "git add"`,
		`gs = "git status"`,
		`ll = "ls -lah"`,
		`la = "ls -A"`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("workspace.toml missing %q\n--- got ---\n%s", want, body)
		}
	}
	if !strings.Contains(string(body), "aliases = {") {
		t.Errorf("expected inline-table aliases line, got:\n%s", body)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_NoAliasBundles_OmitsAliasesLine(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{"--yes", "--service-name", "x", "--username", "y"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --yes: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(work, "workspace.toml"))
	if err != nil {
		t.Fatalf("read workspace.toml: %v", err)
	}
	if strings.Contains(string(body), "aliases =") {
		t.Errorf("expected no aliases line when --alias-bundles unset, got:\n%s", body)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_AliasBundlesFlagRejectsUnknown(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y",
		"--alias-bundles", "k8s",
	})
	if err := cmd.Execute(); !errors.Is(err, ErrUsage) {
		t.Errorf("--alias-bundles k8s should be ErrUsage, got %v", err)
	}
}

// ---------------------------------------------------------------------
// Opt-in section templates (commented-out blocks for discoverability).
// ---------------------------------------------------------------------

// allTemplateSectionHeaders is the canonical list of `# [section]` literals
// every renderWorkspaceToml output must contain. The literals are unchanged
// across locales (TOML schema names are universal) — only the surrounding
// prose is localized — so a single table drives both EN and JA assertions.
var allTemplateSectionHeaders = []string{
	// [container.*]
	"# [container.resources]",
	"# [container.hosts]",
	"# [container.dns]",
	"# [container.sysctls]",
	"# [container.capabilities]",
	"# [container.security_opt]",
	"# [[container.skel]]",
	// [plugins.*]
	"# [plugins.versions]",
	// [apt.*]
	"# [apt.mirror]",
	"# [apt.proxy]",
	"# [[apt.sources]]",
	// top-level
	"# [ports]",
	"# [volumes]",
	"# [env]",
	"# [[mounts]]",
	"# [home_files]",
	"# [locale]",
	"# [dockerfile]",
	"# [services.postgres]",
	"# [devcontainer.customizations.vscode]",
}

func TestRenderWorkspaceToml_AllTemplatesPresent_EN(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", OS: "ubuntu", OSVersion: "26.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
	}, cat)
	for _, header := range allTemplateSectionHeaders {
		if !strings.Contains(got, header) {
			t.Errorf("EN output missing section template %q", header)
		}
	}
}

func TestRenderWorkspaceToml_AllTemplatesPresent_JA(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangJA)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", OS: "ubuntu", OSVersion: "26.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
	}, cat)
	for _, header := range allTemplateSectionHeaders {
		if !strings.Contains(got, header) {
			t.Errorf("JA output missing section template %q", header)
		}
	}
}

// TestRenderWorkspaceToml_NoDeprecatedSections is a regression guard: cocoon
// dropped [git] and [repositories] from the design's intended set. Neither
// must appear as an active section header (`[git]` at line start) nor as
// a commented-out template header (`# [git]` at line start) — both forms
// would nudge a user to use the retired sections.
//
// In-line backrefs ("…replaces the v1 [git] section…") are allowed: they
// document the deprecation rather than promote the section, so we check
// section-header position only, not raw substring presence.
func TestRenderWorkspaceToml_NoDeprecatedSections(t *testing.T) {
	t.Parallel()
	for _, lang := range []i18n.Lang{i18n.LangEN, i18n.LangJA} {
		cat := i18n.New(lang)
		got := renderWorkspaceToml(containerSpec{
			ServiceName: "svc", Username: "dev", OS: "ubuntu", OSVersion: "26.04",
			Shell: "bash", MountRoot: ".", Devcontainer: true,
		}, cat)
		for _, banned := range []string{"[git]", "[repositories]"} {
			for _, prefix := range []string{"", "# "} {
				header := prefix + banned
				for _, line := range strings.Split(got, "\n") {
					if strings.TrimSpace(line) == header {
						t.Errorf("[%s] deprecated section header %q must not appear at line start", lang, header)
					}
				}
			}
		}
	}
}

// TestRenderWorkspaceToml_TemplateOrdering pins that container.* extras
// land between the active [container] block and the active
// [container.shell] block, matching workspace-docker's convention of
// grouping sub-table extras under their parent.
func TestRenderWorkspaceToml_TemplateOrdering(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", OS: "ubuntu", OSVersion: "26.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
	}, cat)

	containerActive := strings.Index(got, "[container]\n")
	containerHostsTpl := strings.Index(got, "# [container.hosts]")
	containerShellActive := strings.Index(got, "[container.shell]\n")
	if containerActive < 0 || containerHostsTpl < 0 || containerShellActive < 0 {
		t.Fatalf("anchor missing: container=%d hosts=%d shell=%d", containerActive, containerHostsTpl, containerShellActive)
	}
	if containerActive >= containerHostsTpl || containerHostsTpl >= containerShellActive {
		t.Errorf("expected order [container] < [container.hosts] template < [container.shell], got %d / %d / %d",
			containerActive, containerHostsTpl, containerShellActive)
	}
}
