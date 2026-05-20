//nolint:testpackage // exercises unexported applyFlags / applyDefaults helpers.
package initcli

import (
	"errors"
	"testing"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
)

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

func TestApplyFlags_DirAccepted(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	for _, dir := range []string{"workspace", "myapp", "work/myapp", "a/b/c"} {
		ans, err := applyFlags(&initFlags{Dir: dir}, plugins)
		if err != nil {
			t.Errorf("--dir %q rejected: %v", dir, err)
			continue
		}
		if ans.Dir != dir || !ans.DirSet {
			t.Errorf("Dir = %q DirSet=%v want %q true", ans.Dir, ans.DirSet, dir)
		}
	}
}

func TestApplyFlags_DirRejected(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	for _, dir := range []string{"/abs", "trail/", "a/../b", "a b", ".."} {
		_, err := applyFlags(&initFlags{Dir: dir}, plugins)
		if !errors.Is(err, clihelpers.ErrUsage) {
			t.Errorf("--dir %q: expected ErrUsage, got %v", dir, err)
		}
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
