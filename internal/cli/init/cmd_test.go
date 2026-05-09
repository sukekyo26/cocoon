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
)

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
	flags := initFlags{
		AutoYes: true, ServiceName: "myapp", Username: "dev",
		OS: "ubuntu", OSVersion: "24.04", MountRoot: "..",
		Devcontainer: false, NoDevcontainer: true,
		AptCategories: "text-editors,build", Force: false,
	}
	ans, err := applyFlags(&flags)
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
	ans, err := applyFlags(&initFlags{})
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
	for _, bad := range []string{"BAD", "with space", "1leading", "_under"} {
		_, err := applyFlags(&initFlags{ServiceName: bad})
		if !errors.Is(err, ErrUsage) {
			t.Errorf("%q → expected ErrUsage, got %v", bad, err)
		}
	}
}

func TestApplyFlags_InvalidUsername(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"BAD", "with space", "1leading"} {
		_, err := applyFlags(&initFlags{Username: bad})
		if !errors.Is(err, ErrUsage) {
			t.Errorf("%q → expected ErrUsage, got %v", bad, err)
		}
	}
}

func TestApplyFlags_InvalidOS(t *testing.T) {
	t.Parallel()
	_, err := applyFlags(&initFlags{OS: "alpine"})
	if !errors.Is(err, ErrUsage) {
		t.Errorf("expected ErrUsage for unknown --os, got %v", err)
	}
}

func TestApplyFlags_OSVersionWithoutOS(t *testing.T) {
	t.Parallel()
	_, err := applyFlags(&initFlags{OSVersion: "24.04"})
	if !errors.Is(err, ErrUsage) {
		t.Errorf("--os-version without --os should be ErrUsage, got %v", err)
	}
}

func TestApplyFlags_OSVersionMismatch(t *testing.T) {
	t.Parallel()
	_, err := applyFlags(&initFlags{OS: "debian", OSVersion: "24.04"})
	if !errors.Is(err, ErrUsage) {
		t.Errorf("ubuntu version on debian should be ErrUsage, got %v", err)
	}
}

func TestApplyFlags_OSVersionValidPair(t *testing.T) {
	t.Parallel()
	ans, err := applyFlags(&initFlags{OS: "debian", OSVersion: "13"})
	if err != nil {
		t.Fatalf("applyFlags: %v", err)
	}
	if ans.OSVersion != "13" || !ans.OSVersionSet {
		t.Errorf("got %+v", ans)
	}
}

func TestApplyFlags_InvalidMountRoot(t *testing.T) {
	t.Parallel()
	_, err := applyFlags(&initFlags{MountRoot: "/abs"})
	if !errors.Is(err, ErrUsage) {
		t.Errorf("expected ErrUsage for /abs mount-root, got %v", err)
	}
}

func TestApplyFlags_DevcontainerExclusivity(t *testing.T) {
	t.Parallel()
	// applyFlags itself does not detect this — runInit does. Confirm that
	// each flag in isolation produces the matching boolean.
	ans, err := applyFlags(&initFlags{Devcontainer: true})
	if err != nil || !ans.Devcontainer || !ans.DevcontainerSet {
		t.Errorf("--devcontainer should set true: %v %+v", err, ans)
	}
	ans, err = applyFlags(&initFlags{NoDevcontainer: true})
	if err != nil || ans.Devcontainer || !ans.DevcontainerSet {
		t.Errorf("--no-devcontainer should set false: %v %+v", err, ans)
	}
}

func TestApplyFlags_UnknownAptCategory(t *testing.T) {
	t.Parallel()
	_, err := applyFlags(&initFlags{AptCategories: "text-editors,not-a-real-category"})
	if !errors.Is(err, ErrUsage) {
		t.Errorf("expected ErrUsage for unknown apt category, got %v", err)
	}
}

func TestApplyFlags_AptCategoriesWhitespace(t *testing.T) {
	t.Parallel()
	ans, err := applyFlags(&initFlags{AptCategories: " text-editors , build , "})
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
	_, err := applyDefaults(initAnswers{Username: "dev"})
	if !errors.Is(err, ErrUsage) {
		t.Errorf("--yes without service_name should be ErrUsage, got %v", err)
	}
}

func TestApplyDefaults_RequiresUsername(t *testing.T) {
	t.Parallel()
	_, err := applyDefaults(initAnswers{ServiceName: "x"})
	if !errors.Is(err, ErrUsage) {
		t.Errorf("--yes without username should be ErrUsage, got %v", err)
	}
}

func TestApplyDefaults_FillsMissingDefaults(t *testing.T) {
	t.Parallel()
	ans, err := applyDefaults(initAnswers{ServiceName: "svc", Username: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if ans.OS != "ubuntu" || !ans.OSSet {
		t.Errorf("OS default = %q OSSet=%v", ans.OS, ans.OSSet)
	}
	if ans.OSVersion != "24.04" || !ans.OSVersionSet {
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
}

func TestApplyDefaults_PreservesExplicitSettings(t *testing.T) {
	t.Parallel()
	in := initAnswers{
		ServiceName: "svc",
		Username:    "dev",
		OS:          "debian", OSSet: true,
		OSVersion: "13", OSVersionSet: true,
		MountRoot: "..", MountRootSet: true,
		Devcontainer: false, DevcontainerSet: true,
		AptCategories: []string{"text-editors"}, AptSet: true,
	}
	ans, err := applyDefaults(in)
	if err != nil {
		t.Fatal(err)
	}
	if ans.OS != "debian" || ans.OSVersion != "13" || ans.MountRoot != ".." ||
		ans.Devcontainer || len(ans.AptCategories) != 1 {
		t.Errorf("explicit values not preserved: %+v", ans)
	}
}

// ---------------------------------------------------------------------
// defaultOSVersion: ubuntu prefers 24.04 LTS; others take the first.
// ---------------------------------------------------------------------

func TestDefaultOSVersion(t *testing.T) {
	t.Parallel()
	if got := defaultOSVersion("ubuntu"); got != "24.04" {
		t.Errorf("ubuntu default = %q, want 24.04", got)
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
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", OS: "ubuntu", OSVersion: "24.04",
		MountRoot: ".", Devcontainer: true, Packages: nil,
	})
	for _, want := range []string{
		`mount_root = "."`,
		`devcontainer = true`,
		`service_name = "svc"`,
		`username = "dev"`,
		`os = "ubuntu"`,
		`os_version = "24.04"`,
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
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", OS: "debian", OSVersion: "13",
		MountRoot: "..", Devcontainer: false, Packages: []string{"vim", "tmux"},
	})
	for _, want := range []string{
		`mount_root = ".."`,
		`devcontainer = false`,
		`os = "debian"`,
		`os_version = "13"`,
		"packages = [\n  \"vim\",\n  \"tmux\",\n]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, got)
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
