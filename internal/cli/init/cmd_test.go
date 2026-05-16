//nolint:testpackage // exercises unexported helpers (applyFlags, applyDefaults, validators, ...).
package initcli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

func loadPluginsForTest(t *testing.T) map[string]*plugin.Plugin {
	t.Helper()
	plugins, err := loadEmbeddedPlugins()
	if err != nil {
		t.Fatalf("loadEmbeddedPlugins: %v", err)
	}
	return plugins
}

// pinEnglish stabilizes assertions on hosts whose LANG starts with "ja".
func pinEnglish(t *testing.T) {
	t.Helper()
	for _, k := range []string{"WORKSPACE_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(k, "")
	}
	t.Setenv("WORKSPACE_LANG", "en")
}

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

func TestApplyFlags_AllValid(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	flags := initFlags{
		AutoYes: true, ServiceName: "myapp", Username: "dev",
		Image: "ubuntu", ImageVersion: "24.04", MountRoot: "..",
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
	if ans.Image != "ubuntu" || !ans.ImageSet {
		t.Errorf("Image = %q ImageSet=%v", ans.Image, ans.ImageSet)
	}
	if ans.ImageVersion != "24.04" || !ans.ImageVersionSet {
		t.Errorf("ImageVersion = %q ImageVersionSet=%v", ans.ImageVersion, ans.ImageVersionSet)
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
	if ans.ServiceName != "" || ans.Username != "" || ans.ImageSet || ans.ImageVersionSet ||
		ans.MountRootSet || ans.DevcontainerSet || ans.AptSet {
		t.Errorf("expected fully zero answers, got %+v", ans)
	}
}

func TestApplyFlags_InvalidServiceName(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	for _, bad := range []string{"BAD", "with space", "1leading", "_under"} {
		_, err := applyFlags(&initFlags{ServiceName: bad}, plugins)
		if !errors.Is(err, clihelpers.ErrUsage) {
			t.Errorf("%q → expected ErrUsage, got %v", bad, err)
		}
	}
}

func TestApplyFlags_InvalidUsername(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	for _, bad := range []string{"BAD", "with space", "1leading"} {
		_, err := applyFlags(&initFlags{Username: bad}, plugins)
		if !errors.Is(err, clihelpers.ErrUsage) {
			t.Errorf("%q → expected ErrUsage, got %v", bad, err)
		}
	}
}

func TestApplyFlags_InvalidImage(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	_, err := applyFlags(&initFlags{Image: "alpine"}, plugins)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("expected ErrUsage for unknown --image, got %v", err)
	}
}

func TestApplyFlags_ImageVersionWithoutImage(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	_, err := applyFlags(&initFlags{ImageVersion: "24.04"}, plugins)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("--image-version without --image should be ErrUsage, got %v", err)
	}
}

// TestApplyFlags_ImageVersionBadFormat: spaces, slashes, colons must be
// rejected at the flag layer so the user sees a clear error before
// `cocoon gen` runs.
func TestApplyFlags_ImageVersionBadFormat(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	for _, bad := range []string{"with space", "ubuntu/22.04", "node:24", "tab\there", ""} {
		if bad == "" {
			continue // empty means "flag not set", which is allowed
		}
		_, err := applyFlags(&initFlags{Image: "ubuntu", ImageVersion: bad}, plugins)
		if !errors.Is(err, clihelpers.ErrUsage) {
			t.Errorf("bad image-version %q should be ErrUsage, got %v", bad, err)
		}
	}
}

// TestApplyFlags_ImageVersionAcceptsOffWhitelist: any tag matching
// rxImageVersionInput is accepted even when not in SupportedImageVersions.
// This lets users pin patch tags or new minors without waiting for a release.
func TestApplyFlags_ImageVersionAcceptsOffWhitelist(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	for _, tag := range []string{"1.26.4-bookworm", "26-bookworm-slim", "edge"} {
		ans, err := applyFlags(&initFlags{Image: "golang", ImageVersion: tag}, plugins)
		if err != nil {
			t.Errorf("off-whitelist tag %q should be accepted, got %v", tag, err)
			continue
		}
		if ans.ImageVersion != tag || !ans.ImageVersionSet {
			t.Errorf("got %+v", ans)
		}
	}
}

func TestApplyFlags_ImageVersionWhitelistedPair(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	ans, err := applyFlags(&initFlags{Image: "debian", ImageVersion: "13"}, plugins)
	if err != nil {
		t.Fatalf("applyFlags: %v", err)
	}
	if ans.ImageVersion != "13" || !ans.ImageVersionSet {
		t.Errorf("got %+v", ans)
	}
}

func TestApplyFlags_InvalidMountRoot(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	_, err := applyFlags(&initFlags{MountRoot: "/abs"}, plugins)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("expected ErrUsage for /abs mount-root, got %v", err)
	}
}

func TestApplyFlags_DevcontainerExclusivity(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	// applyFlags itself does not detect the mutual exclusivity — runInit
	// does. Confirm each flag in isolation produces the matching boolean.
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
	if !errors.Is(err, clihelpers.ErrUsage) {
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

func TestApplyDefaults_RequiresServiceName(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	_, err := applyDefaults(initAnswers{Username: "dev"}, plugins)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("--yes without service_name should be ErrUsage, got %v", err)
	}
}

func TestApplyDefaults_RequiresUsername(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	_, err := applyDefaults(initAnswers{ServiceName: "x"}, plugins)
	if !errors.Is(err, clihelpers.ErrUsage) {
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
	if ans.Image != "ubuntu" || !ans.ImageSet {
		t.Errorf("Image default = %q ImageSet=%v", ans.Image, ans.ImageSet)
	}
	if ans.ImageVersion != "26.04" || !ans.ImageVersionSet {
		t.Errorf("ImageVersion default = %q", ans.ImageVersion)
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
		Image:       "debian", ImageSet: true,
		ImageVersion: "13", ImageVersionSet: true,
		MountRoot: "..", MountRootSet: true,
		Devcontainer: false, DevcontainerSet: true,
		AptCategories: []string{"text-editors"}, AptSet: true,
	}
	ans, err := applyDefaults(in, plugins)
	if err != nil {
		t.Fatal(err)
	}
	if ans.Image != "debian" || ans.ImageVersion != "13" || ans.MountRoot != ".." ||
		ans.Devcontainer || len(ans.AptCategories) != 1 {
		t.Errorf("explicit values not preserved: %+v", ans)
	}
}

func TestDefaultImageVersion(t *testing.T) {
	t.Parallel()
	if got := defaultImageVersion("ubuntu"); got != "26.04" {
		t.Errorf("ubuntu default = %q, want 26.04", got)
	}
	if got := defaultImageVersion("debian"); got != "13" {
		t.Errorf("debian default = %q, want 13", got)
	}
	if got := defaultImageVersion("alpine"); got != "" {
		t.Errorf("unknown OS default should be \"\", got %q", got)
	}
}

// TestFilterPluginIDs pins the three shapes the picker relies on so the
// same id never surfaces in both the default list and the excluded list.
func TestFilterPluginIDs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		in        []string
		excludeID string
		want      []string
	}{
		{"no_exclude_returns_input", []string{"a", "b", "c"}, "", []string{"a", "b", "c"}},
		{"excludes_present_id", []string{"a", "rust", "c"}, "rust", []string{"a", "c"}},
		{"absent_id_is_noop", []string{"a", "b"}, "rust", []string{"a", "b"}},
		{"empty_input", []string{}, "rust", []string{}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filterPluginIDs(tc.in, tc.excludeID)
			if len(got) != len(tc.want) {
				t.Fatalf("len=%d, want %d, got=%v", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("at %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestPluginsMultiSelect_BuildsForEveryExcludeID is a smoke test: huh's
// option list isn't reachable through a stable API, so this only confirms
// construction does not panic. Exclusion behavior itself is covered by
// TestFilterPluginIDs.
func TestPluginsMultiSelect_BuildsForEveryExcludeID(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	cat := i18n.New(i18n.LangEN)
	var target []string

	for _, excludeID := range []string{"", "rust", "go", "node", "deno"} {
		excludeID := excludeID
		t.Run("exclude="+excludeID, func(t *testing.T) {
			t.Parallel()
			sel := pluginsMultiSelect(cat, plugins, excludeID, &target)
			if sel == nil {
				t.Fatal("pluginsMultiSelect returned nil")
			}
		})
	}
}

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
	if _, err := parseAptCategories("not-a-real-cat"); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("unknown category should be ErrUsage, got %v", err)
	}
}

func TestRenderWorkspaceToml_NoPackages(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "24.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true, Packages: nil,
	}, cat)
	for _, want := range []string{
		`mount_root = "."`,
		`devcontainer = true`,
		`service_name = "svc"`,
		`username = "dev"`,
		`image = "ubuntu"`,
		`image_version = "24.04"`,
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
		ServiceName: "svc", Username: "dev", Image: "debian", ImageVersion: "13",
		Shell: "zsh", MountRoot: "..", Devcontainer: false,
		Packages: []string{"vim", "tmux"},
	}, cat)
	for _, want := range []string{
		`mount_root = ".."`,
		`devcontainer = false`,
		`image = "debian"`,
		`image_version = "13"`,
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
		ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "24.04",
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
		ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "24.04",
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
		ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "24.04",
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
		ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "24.04",
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
		ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "24.04",
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
		ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "24.04",
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
			ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "24.04",
			Shell: "bash", MountRoot: ".", Devcontainer: true,
		}, cat)
		for _, want := range []string{"text-editors", "VS Code", "utilities"} {
			if !strings.Contains(got, want) {
				t.Errorf("[%s] caveat keyword %q missing\n--- got ---\n%s", lang, want, got)
			}
		}
	}
}

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
		"--image", "ubuntu", "--image-version", "24.04",
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
		`image = "ubuntu"`, `image_version = "24.04"`,
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
	if !errors.Is(err, clihelpers.ErrUsage) {
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
	if !errors.Is(err, clihelpers.ErrUsage) {
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
	if err := cmd.Execute(); !errors.Is(err, clihelpers.ErrUsage) {
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
	if err := cmd.Execute(); !errors.Is(err, clihelpers.ErrUsage) {
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
	if err := cmd.Execute(); !errors.Is(err, clihelpers.ErrUsage) {
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
				"--image", "ubuntu", "--image-version", "22.04",
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
	if _, err := parsePlugins("does-not-exist", plugins); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("unknown plugin should be ErrUsage, got %v", err)
	}
}

func TestValidatePluginConflicts(t *testing.T) {
	t.Parallel()
	// The embedded catalog currently ships no plugins with declared
	// conflicts (custom-ps1 was the only one and has been removed). Build
	// a synthetic pair in-memory so the validator's symmetric-detection
	// logic still has a regression guard.
	plugins := map[string]*plugin.Plugin{
		"alpha": {Metadata: plugin.Metadata{Name: "Alpha", Conflicts: []string{"beta"}}},
		"beta":  {Metadata: plugin.Metadata{Name: "Beta", Conflicts: []string{"alpha"}}},
		"gamma": {Metadata: plugin.Metadata{Name: "Gamma"}},
	}

	err := validatePluginConflicts(plugins, []string{"alpha", "beta"})
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("alpha+beta should be ErrUsage, got %v", err)
	}

	// Either alone is fine.
	if err := validatePluginConflicts(plugins, []string{"alpha"}); err != nil {
		t.Errorf("alpha alone should be ok, got %v", err)
	}
	if err := validatePluginConflicts(plugins, []string{"beta"}); err != nil {
		t.Errorf("beta alone should be ok, got %v", err)
	}

	// Empty list is trivially ok.
	if err := validatePluginConflicts(plugins, nil); err != nil {
		t.Errorf("nil enabled should be ok, got %v", err)
	}

	// Disjoint plugins do not conflict.
	if err := validatePluginConflicts(plugins, []string{"alpha", "gamma"}); err != nil {
		t.Errorf("disjoint plugins should be ok, got %v", err)
	}
}

func TestDefaultPluginIDs(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)

	got := defaultPluginIDs(plugins)
	// The current catalog ships no `default = true` plugin. If you add one,
	// update this assertion alongside the catalog change.
	if len(got) != 0 {
		t.Errorf("default plugin ids: got %v, want none", got)
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
	if err := cmd.Execute(); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("unknown plugin should be ErrUsage, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(work, "workspace.toml")); statErr == nil {
		t.Error("workspace.toml should NOT have been written when --plugins lists an unknown id")
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_YesNoDefaultPlugins(t *testing.T) {
	// `--yes` with no --plugins falls back to defaultPluginIDs(); the
	// embedded catalog ships no `default = true` plugin, so the generated
	// workspace.toml enables nothing.
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
	want := "[plugins]\nenable = []"
	if !strings.Contains(string(body), want) {
		t.Errorf("workspace.toml missing %q\n--- got ---\n%s", want, body)
	}
}

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
		if _, err := parsePluginVersions(bad, plugins, enabled); !errors.Is(err, clihelpers.ErrUsage) {
			t.Errorf("malformed token %q: expected ErrUsage, got %v", bad, err)
		}
	}

	if _, err := parsePluginVersions("does-not-exist=1.0", plugins, enabled); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("unknown plugin: expected ErrUsage, got %v", err)
	}

	// docker-cli ships in the embedded catalog but is `version_capable=false`.
	if _, err := parsePluginVersions("docker-cli=1.0", plugins, []string{"docker-cli"}); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("non-version-capable: expected ErrUsage, got %v", err)
	}

	// `go` is version_capable but the caller did not list it in --plugins.
	if _, err := parsePluginVersions("go=1.23.4", plugins, []string{"uv"}); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("missing-from-enable: expected ErrUsage, got %v", err)
	}

	if _, err := parsePluginVersions("go=1.23.4,go=1.24.0", plugins, enabled); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("duplicate id: expected ErrUsage, got %v", err)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_PluginVersionsFlagWritesInlineLines(t *testing.T) {
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
	// Sorted by id: go before starship; one [plugins.versions] section header
	// plus an inline-table line per pin (no checksums emitted from --plugin-versions).
	want := "[plugins.versions]\ngo = { pin = \"1.23.4\" }\nstarship = { pin = \"1.21.1\" }\n"
	if !strings.Contains(string(body), want) {
		t.Errorf("workspace.toml missing pin lines\n--- want ---\n%s\n--- got ---\n%s", want, body)
	}
	// The commented-out example template must NOT appear when real pins
	// were emitted — otherwise the user sees both the example and their own
	// entries. The template's leading comment is unique to the example.
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
	if err := cmd.Execute(); !errors.Is(err, clihelpers.ErrUsage) {
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
	if err := cmd.Execute(); !errors.Is(err, clihelpers.ErrUsage) {
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
	if err := cmd.Execute(); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("expected ErrUsage when pin is for a non-enabled plugin, got %v", err)
	}
}

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
		if !errors.Is(err, clihelpers.ErrUsage) {
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
	if err := cmd.Execute(); !errors.Is(err, clihelpers.ErrUsage) {
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
	if _, err := parseAliasBundles("k8s"); !errors.Is(err, clihelpers.ErrUsage) {
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
	if err := cmd.Execute(); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("--alias-bundles k8s should be ErrUsage, got %v", err)
	}
}

func TestParsePorts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{"empty_returns_nil", "", nil, false},
		{"whitespace_only_returns_nil", " , ,\t,", nil, false},
		{"single_short", "3000:3000", []string{"3000:3000"}, false},
		{"multiple_short", "3000:3000,5432:5432", []string{"3000:3000", "5432:5432"}, false},
		{"trims_whitespace", " 3000:3000 , 5432:5432 ", []string{"3000:3000", "5432:5432"}, false},
		{
			"accepts_all_documented_forms",
			"3000,3000-3005,8000:8000,9090-9091:8080-8081,49100:22," +
				"127.0.0.1:8001:8001,127.0.0.1:5000-5010:5000-5010,6060:6060/udp",
			[]string{
				"3000", "3000-3005", "8000:8000", "9090-9091:8080-8081", "49100:22",
				"127.0.0.1:8001:8001", "127.0.0.1:5000-5010:5000-5010", "6060:6060/udp",
			},
			false,
		},
		{"rejects_garbage", "abc", nil, true},
		{"rejects_out_of_range", "99999:80", nil, true},
		{"rejects_bad_ip", "999.999.999.999:80:80", nil, true},
		{"rejects_unknown_proto", "3000:3000/sctp", nil, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePorts(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parsePorts(%q) err = nil, want error", tc.raw)
				}
				if !errors.Is(err, clihelpers.ErrUsage) {
					t.Errorf("parsePorts(%q) err = %v, want errors.Is ErrUsage", tc.raw, err)
				}
				if !errors.Is(err, config.ErrPortShortForm) {
					t.Errorf("parsePorts(%q) err = %v, want errors.Is config.ErrPortShortForm", tc.raw, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePorts(%q) err = %v, want nil", tc.raw, err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("parsePorts(%q) mismatch (-want +got):\n%s", tc.raw, diff)
			}
		})
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_PortsFlagWritesActiveBlock(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y",
		"--ports", "3000:3000,5432:5432",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --ports: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(work, "workspace.toml"))
	if err != nil {
		t.Fatalf("read workspace.toml: %v", err)
	}
	got := string(body)
	for _, want := range []string{
		"\n[ports]\n",
		"forward = [\n    \"3000:3000\",\n    \"5432:5432\",\n]\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("workspace.toml missing %q\n--- got ---\n%s", want, got)
		}
	}
	// Active block replaces the commented template; the literal example
	// "# forward = [\"3000:3000\", \"5432:5432\"]" must not also appear,
	// or readers would see two competing port lists.
	if strings.Contains(got, "# forward = [\"3000:3000\", \"5432:5432\"]") {
		t.Errorf("active --ports should suppress the commented template, got:\n%s", got)
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_NoPorts_EmitsCommentedTemplate(t *testing.T) {
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
	got := string(body)
	// No --ports = template commented out; no live `forward = [...]`
	// (the active line would lack a leading `#`).
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "forward = ") &&
			!strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("unexpected active forward line: %q", line)
		}
	}
	if !strings.Contains(got, "# forward = [\"3000:3000\", \"5432:5432\"]") {
		t.Errorf("expected commented [ports] template to remain when --ports unset, got:\n%s", got)
	}
}

// TestPortsInputValidator pins the i18n behavior of the interactive prompt
// validator: rejection messages come from the catalog (EN / JA), not from
// config.ValidateShortForm's English text. Accept paths return nil so huh
// advances to the next group.
func TestPortsInputValidator(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		lang i18n.Lang
		in   string
		// substring asserted in err.Error(); empty = nil expected.
		wantSubstr string
	}{
		{"en_accept_blank", i18n.LangEN, "", ""},
		{"en_accept_single", i18n.LangEN, "3000:3000", ""},
		{
			"en_accept_all_forms", i18n.LangEN,
			"3000,3000-3005,8000:8000,127.0.0.1:8001:8001,6060:6060/udp", "",
		},
		{
			"en_reject_uses_catalog_phrase", i18n.LangEN, "abc",
			"is not a valid port short form",
		},
		{
			"ja_reject_uses_catalog_phrase", i18n.LangJA, "abc",
			"はポート指定として無効です",
		},
		{
			"ja_reject_out_of_range", i18n.LangJA, "99999:80",
			"はポート指定として無効です",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cat := i18n.New(tc.lang)
			err := portsInputValidator(cat)(tc.in)
			if tc.wantSubstr == "" {
				if err != nil {
					t.Fatalf("validator(%q) = %v, want nil", tc.in, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validator(%q) = nil, want error containing %q", tc.in, tc.wantSubstr)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("validator(%q) err = %q, want substring %q",
					tc.in, err.Error(), tc.wantSubstr)
			}
		})
	}
}

//nolint:paralleltest // t.Chdir
func TestRunInit_PortsFlagRejectsInvalid(t *testing.T) {
	pinEnglish(t)
	cases := []struct {
		name string
		flag string
	}{
		{"garbage", "abc"},
		{"out_of_range", "99999:80"},
		{"bad_ip", "999.999.999.999:80:80"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			work := t.TempDir()
			t.Chdir(work)
			cmd := NewCommand(io.Discard, io.Discard)
			cmd.SetArgs([]string{
				"--yes", "--service-name", "x", "--username", "y",
				"--ports", tc.flag,
			})
			if err := cmd.Execute(); !errors.Is(err, clihelpers.ErrUsage) {
				t.Errorf("--ports %q should be ErrUsage, got %v", tc.flag, err)
			}
		})
	}
}

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
		ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "26.04",
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
		ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "26.04",
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
			ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "26.04",
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
// [container.shell] block, grouping sub-table extras under their parent.
func TestRenderWorkspaceToml_TemplateOrdering(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "26.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
	}, cat)

	containerActive := strings.Index(got, "[container]\n")
	dockerSocketTpl := strings.Index(got, "# docker_socket = true")
	groupAddTpl := strings.Index(got, "# group_add = [")
	containerHostsTpl := strings.Index(got, "# [container.hosts]")
	containerShellActive := strings.Index(got, "[container.shell]\n")
	if containerActive < 0 || dockerSocketTpl < 0 || groupAddTpl < 0 ||
		containerHostsTpl < 0 || containerShellActive < 0 {
		t.Fatalf("anchor missing: container=%d docker_socket=%d group_add=%d hosts=%d shell=%d",
			containerActive, dockerSocketTpl, groupAddTpl, containerHostsTpl, containerShellActive)
	}
	if containerActive >= dockerSocketTpl || dockerSocketTpl >= groupAddTpl ||
		groupAddTpl >= containerHostsTpl || containerHostsTpl >= containerShellActive {
		t.Errorf("expected order [container] < docker_socket < group_add < [container.hosts] < [container.shell], "+
			"got %d / %d / %d / %d / %d",
			containerActive, dockerSocketTpl, groupAddTpl, containerHostsTpl, containerShellActive)
	}
}

// TestRenderWorkspaceToml_DockerSocketTemplatePresent pins that every
// generated workspace.toml carries the commented-out docker_socket opt-in
// line so users discover it without re-running init.
func TestRenderWorkspaceToml_DockerSocketTemplatePresent(t *testing.T) {
	t.Parallel()
	for _, lang := range []i18n.Lang{i18n.LangEN, i18n.LangJA} {
		cat := i18n.New(lang)
		got := renderWorkspaceToml(containerSpec{
			ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "26.04",
			Shell: "bash", MountRoot: ".", Devcontainer: true,
		}, cat)
		if !strings.Contains(got, "# docker_socket = true") {
			t.Errorf("[%s] output missing docker_socket template line\n--- got ---\n%s", lang, got)
		}
	}
}
