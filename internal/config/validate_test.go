package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/config"
)

// minimalWorkspace returns a TOML string for a syntactically valid workspace
// with the smallest possible content. Tests append extra sections to it.
func minimalWorkspace() string {
	return `
[container]
service_name = "dev"
username = "developer"
image = "ubuntu"
image_version = "24.04"

[plugins]
enable = []
`
}

func loadWS(t *testing.T, body string) error {
	t.Helper()
	tmp := t.TempDir() + "/ws.toml"
	require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
	_, err := config.LoadWorkspace(tmp)
	return err
}

func TestValidate_ContainerInvalidServiceName(t *testing.T) {
	t.Parallel()
	body := strings.ReplaceAll(minimalWorkspace(), `service_name = "dev"`, `service_name = "Dev"`)
	err := loadWS(t, body)
	require.Error(t, err)
	v, ok := config.AsValidationError(err)
	require.True(t, ok)
	require.Contains(t, v.Errors[0].Message, "service_name does not match")
}

// TestValidate_LegacyOsRejected pins the migration error message so the
// rewrite snippet does not silently regress. A workspace.toml that still
// uses the pre-v0.3 `os = "..." / os_version = "..."` fields must emit a
// container.os error containing both the new field names (image /
// image_version) and the original values, so the user can copy the snippet
// straight from the error and re-run.
func TestValidate_LegacyOsRejected(t *testing.T) {
	t.Parallel()
	body := strings.ReplaceAll(
		minimalWorkspace(),
		`image = "ubuntu"
image_version = "24.04"`,
		`os = "ubuntu"
os_version = "24.04"`,
	)
	err := loadWS(t, body)
	require.Error(t, err)
	v, ok := config.AsValidationError(err)
	require.True(t, ok)
	var got *config.FieldError
	for i := range v.Errors {
		if len(v.Errors[i].Loc) > 0 && v.Errors[i].Loc[len(v.Errors[i].Loc)-1] == "os" {
			got = &v.Errors[i]
			break
		}
	}
	require.NotNilf(t, got, "no os deprecation error in: %v", v.Errors)
	for _, want := range []string{
		"os / os_version are no longer supported",
		`image = "ubuntu"`,
		`image_version = "24.04"`,
	} {
		require.Containsf(t, got.Message, want, "migration error must contain %q", want)
	}
}

// TestValidate_LegacyOsSuppressesImageErrors verifies that when the legacy
// `os` / `os_version` fields are present (and image / image_version are
// therefore missing), the migration error is the only container-image
// error emitted. Stacking "image is required" / "image_version is required"
// on top of the migration snippet would bury the actionable rewrite.
func TestValidate_LegacyOsSuppressesImageErrors(t *testing.T) {
	t.Parallel()
	body := strings.ReplaceAll(
		minimalWorkspace(),
		`image = "ubuntu"
image_version = "24.04"`,
		`os = "ubuntu"
os_version = "24.04"`,
	)
	err := loadWS(t, body)
	require.Error(t, err)
	v, ok := config.AsValidationError(err)
	require.True(t, ok)
	for i := range v.Errors {
		loc := v.Errors[i].Loc
		if len(loc) == 0 {
			continue
		}
		last := loc[len(loc)-1]
		if last == "image" && !strings.Contains(v.Errors[i].Message, "no longer supported") {
			t.Errorf("validateImage error must be suppressed while os/os_version is set, got %v: %q", loc, v.Errors[i].Message)
		}
		if last == "image_version" {
			t.Errorf("validateImage error must be suppressed while os/os_version is set, got %v: %q", loc, v.Errors[i].Message)
		}
	}
}

func TestValidate_DuplicatePlugins(t *testing.T) {
	t.Parallel()
	body := strings.ReplaceAll(minimalWorkspace(), "enable = []", `enable = ["go", "go"]`)
	err := loadWS(t, body)
	require.Error(t, err)
	v, _ := config.AsValidationError(err)
	require.Contains(t, v.Error(), "duplicate")
}

// TestValidate_ImageWhitelist exercises validateImage across all seven
// supported images and every entry in their per-image SupportedImageVersions
// list. The point is two-fold: catch a future SupportedImageVersions edit
// that desynchronises from validateImage's lookup, and catch a future
// SupportedImages edit that adds an entry without the corresponding
// SupportedImageVersions row. The closed set is small enough that exhausting
// it here costs <1ms but keeps the validator and the whitelist in lockstep.
func TestValidate_ImageWhitelist(t *testing.T) {
	t.Parallel()
	for _, image := range config.SupportedImages {
		image := image
		for _, version := range config.SupportedImageVersions[image] {
			version := version
			t.Run(image+"/"+version, func(t *testing.T) {
				t.Parallel()
				body := strings.ReplaceAll(
					strings.ReplaceAll(
						minimalWorkspace(),
						`image = "ubuntu"`,
						`image = "`+image+`"`,
					),
					`image_version = "24.04"`,
					`image_version = "`+version+`"`,
				)
				require.NoError(t, loadWS(t, body))
			})
		}
	}
}

// TestValidate_ImageRejected covers every way validateImage emits an
// error: unknown image id, missing version when the image is set, badly
// formatted version (slash, space, colon — anything outside
// rxImageVersion), and missing image with a version set. Each case
// asserts on the message substring users actually see (not on a
// sentinel — validate.go uses FieldError, not exported sentinels).
//
// Note: an off-whitelist but well-formed tag (e.g. "1.26.4-bookworm")
// is intentionally NOT rejected — that's TestValidate_ImageAcceptsOffWhitelist.
func TestValidate_ImageRejected(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		imageLine   string
		versionLine string
		mustContain string
	}{
		{
			name:        "unknown_image",
			imageLine:   `image = "alpine"`,
			versionLine: `image_version = "3.20"`,
			mustContain: "image must be one of",
		},
		{
			name:        "image_version_bad_format_slash",
			imageLine:   `image = "node"`,
			versionLine: `image_version = "library/node:24"`,
			mustContain: "does not match",
		},
		{
			name:        "image_version_bad_format_space",
			imageLine:   `image = "node"`,
			versionLine: `image_version = "with space"`,
			mustContain: "does not match",
		},
		{
			name:        "image_version_bad_format_colon",
			imageLine:   `image = "node"`,
			versionLine: `image_version = "24:bookworm"`,
			mustContain: "does not match",
		},
		{
			name:        "image_version_missing",
			imageLine:   `image = "node"`,
			versionLine: `image_version = ""`,
			mustContain: "image_version is required for image=node",
		},
		{
			name:        "image_missing",
			imageLine:   `image = ""`,
			versionLine: `image_version = "26.04"`,
			mustContain: "image is required",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := strings.ReplaceAll(
				strings.ReplaceAll(minimalWorkspace(),
					`image = "ubuntu"`, tc.imageLine),
				`image_version = "24.04"`, tc.versionLine)
			err := loadWS(t, body)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.mustContain)
		})
	}
}

// TestValidate_ImageAcceptsOffWhitelist pins the relaxed behavior: any
// tag matching rxImageVersion is accepted even when SupportedImageVersions
// does not list it. That lets users pin patches / new minors the day
// upstream publishes them, instead of waiting for a cocoon release to
// extend the whitelist.
func TestValidate_ImageAcceptsOffWhitelist(t *testing.T) {
	t.Parallel()
	cases := []struct {
		image   string
		version string
	}{
		{"go", "1.26.4-bookworm"},    // hypothetical future patch
		{"go", "1.99-bookworm"},      // hypothetical future minor
		{"node", "27-bookworm-slim"}, // hypothetical future major
		{"python", "3.15-slim-bookworm"},
		{"deno", "debian-2.7.99"},
		{"ubuntu", "27.04"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.image+"_"+tc.version, func(t *testing.T) {
			t.Parallel()
			body := strings.ReplaceAll(
				strings.ReplaceAll(minimalWorkspace(),
					`image = "ubuntu"`,
					`image = "`+tc.image+`"`),
				`image_version = "24.04"`,
				`image_version = "`+tc.version+`"`)
			require.NoError(t, loadWS(t, body))
		})
	}
}

// TestValidate_ImagePluginConflict exercises the cross-section check that
// rejects workspace.toml files combining a language-runtime base image
// with the matching cocoon plugin. The conflict pairs come from
// ImageProvidesPlugin (image="go" ↔ plugin "go", image="rust" ↔ plugin
// "rust"); other combinations (image="python" + uv plugin, image="ubuntu"
// + go plugin) must remain accepted because they coexist cleanly.
func TestValidate_ImagePluginConflict(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		image       string
		version     string
		enable      string
		wantErr     bool
		mustContain string
	}{
		{
			name:  "go_image_plus_go_plugin",
			image: "go", version: "1.26-bookworm",
			enable:      `["go"]`,
			wantErr:     true,
			mustContain: `image = "go" already provides go`,
		},
		{
			name:  "rust_image_plus_rust_plugin",
			image: "rust", version: "1.95-bookworm",
			enable:      `["rust"]`,
			wantErr:     true,
			mustContain: `image = "rust" already provides rust`,
		},
		{
			name:  "ubuntu_image_plus_go_plugin_ok",
			image: "ubuntu", version: "24.04",
			enable:  `["go"]`,
			wantErr: false,
		},
		{
			name:  "python_image_plus_uv_plugin_ok",
			image: "python", version: "3.13-slim-bookworm",
			enable:  `["uv"]`,
			wantErr: false,
		},
		{
			name:  "go_image_alone_ok",
			image: "go", version: "1.26-bookworm",
			enable:  `[]`,
			wantErr: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := strings.ReplaceAll(
				strings.ReplaceAll(
					strings.ReplaceAll(minimalWorkspace(),
						`image = "ubuntu"`,
						`image = "`+tc.image+`"`),
					`image_version = "24.04"`,
					`image_version = "`+tc.version+`"`),
				"enable = []",
				"enable = "+tc.enable)
			err := loadWS(t, body)
			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.mustContain)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestResolveImageRegistry pins the registry-path mapping used by both the
// Dockerfile FROM line and the .env IMAGE= entry. Every supported image
// either resolves verbatim (library/<id>) or appears as an explicit
// ImageRegistryPath override — currently only deno. Adding or removing a
// vendor-namespaced image is therefore a one-line edit to the map and a
// matching case here.
func TestResolveImageRegistry(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"ubuntu": "ubuntu",
		"debian": "debian",
		"node":   "node",
		"python": "python",
		"go":     "golang",
		"rust":   "rust",
		"deno":   "denoland/deno",
	}
	for image, want := range cases {
		image, want := image, want
		t.Run(image, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, want, config.ResolveImageRegistry(image))
		})
	}
}

func TestValidate_InvalidEnvKey(t *testing.T) {
	t.Parallel()
	err := loadWS(t, minimalWorkspace()+"\n[env]\n\"123BAD\" = \"v\"\n")
	require.Error(t, err)
}

func TestValidate_ShellInvalidAliasKey(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[container.shell]\naliases = { \"$bad$\" = \"ls\" }\nenv = {}\n"
	err := loadWS(t, body)
	require.Error(t, err)
}

func TestValidate_LegacyShellSectionGivesMigrationHint(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[shell]\naliases = { ll = \"ls -lah\" }\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "[container.shell]")
}

func TestValidate_ContainerShellInvalidDefault(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[container.shell]\ndefault = \"nu\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "default")
}

func TestValidate_ContainerShellAcceptsFish(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[container.shell]\ndefault = \"fish\"\nenv = { EDITOR = \"vim\" }\n"
	err := loadWS(t, body)
	require.NoError(t, err)
}

func TestValidate_SidecarCollidesWithMain(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[services.dev]\nimage = \"redis:7\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "collides")
}

func TestValidate_DependsOnMain(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[services.redis]\nimage = \"redis:7\"\ndepends_on = [\"dev\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "main service")
}

func TestValidate_DependsOnUndefined(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[services.redis]\nimage = \"redis:7\"\ndepends_on = [\"ghost\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "undefined sidecar")
}

func TestValidate_DependsOnSelf(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[services.redis]\nimage = \"redis:7\"\ndepends_on = [\"redis\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "references itself")
}

func TestValidate_ReservedLocalVolume(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[services.redis]\nimage = \"redis:7\"\nvolumes = { local = \"/data\" }\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), `reserved name "local"`)
}

func TestValidate_RepoPathDotDot(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[repositories]\nclone = [{ url = \"https://x/y/foo.git\", path = \"foo/../bar\" }]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "`..` segments")
}

func TestValidate_RepoPathBadCharacter(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[repositories]\nclone = [{ url = \"https://x/y/foo.git\", path = \"with space\" }]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), `[A-Za-z0-9_./-]`)
}

func TestValidate_RepoPathDotDotSubstring(t *testing.T) {
	t.Parallel()
	// "foo..bar" contains ".." as a substring but not as a path segment,
	// so it must pass validation (no path traversal risk).
	body := minimalWorkspace() +
		"\n[repositories]\nclone = [{ url = \"https://x/y/foo.git\", path = \"foo..bar\" }]\n"
	err := loadWS(t, body)
	require.NoError(t, err)
}

func TestValidate_RepoWorkspaceDocker(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[repositories]\nclone = [{ url = \"https://x/y/workspace-docker.git\" }]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "workspace-docker itself")
}

func TestValidate_RepoDuplicatePath(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[repositories]\nclone = [\n" +
		"  { url = \"https://x/y/foo.git\" },\n" +
		"  { url = \"https://other/foo.git\" },\n]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "collides with entry")
}

func TestValidate_RepoDuplicateURL(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[repositories]\nclone = [\n" +
		"  { url = \"https://x/y/foo.git\", path = \"a\" },\n" +
		"  { url = \"https://x/y/foo.git\", path = \"b\" },\n]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicates entry")
}

func TestValidate_UnresolvableRepoPath(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[repositories]\nclone = [{ url = \"?\", path = \"libs/\" }]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot derive target path")
}

func TestValidate_SidecarInvalidEnvKey(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[services.redis]\nimage = \"redis:7\"\nenv = { \"1BAD\" = \"v\" }\n"
	err := loadWS(t, body)
	require.Error(t, err)
}

func TestValidate_SidecarDuplicateDepends(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[services.a]\nimage = \"x:1\"\n" +
		"\n[services.b]\nimage = \"y:1\"\ndepends_on = [\"a\", \"a\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
}

func TestValidate_HomeFilesValid(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[home_files]\nfiles = [\".claude.json\", \".gemini/oauth_creds.json\"]\n"
	err := loadWS(t, body)
	require.NoError(t, err)
}

func TestValidate_HomeFilesRejectsAbsolute(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[home_files]\nfiles = [\"/etc/passwd\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no leading /")
}

func TestValidate_HomeFilesRejectsTilde(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[home_files]\nfiles = [\"~/.claude.json\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "~")
}

func TestValidate_HomeFilesRejectsDotDot(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[home_files]\nfiles = [\"../escape\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "../")
}

func TestValidate_HomeFilesRejectsDotDotSegment(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[home_files]\nfiles = [\".cfg/../escape\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "`..`")
}

func TestValidate_HomeFilesRejectsTrailingSlash(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[home_files]\nfiles = [\".config/\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "files only")
}

func TestValidate_HomeFilesRejectsEmpty(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[home_files]\nfiles = [\"\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not be empty")
}

func TestValidate_HomeFilesRejectsColon(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[home_files]\nfiles = [\".cache/foo:bar.json\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "`:`")
}

func TestValidate_HomeFilesRejectsDuplicate(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[home_files]\nfiles = [\".claude.json\", \".claude.json\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

func TestValidate_ContainerHostsAcceptsValid(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.hosts]\n\"db.local\" = \"host-gateway\"\n\"corp.example\" = \"10.0.0.42\"\n"
	require.NoError(t, loadWS(t, body))
}

func TestValidate_ContainerHostsRejectsBadHostname(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.hosts]\n\"bad host name\" = \"127.0.0.1\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "hostname does not match")
}

func TestValidate_ContainerHostsRejectsBadIP(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.hosts]\n\"host.local\" = \"not-an-ip\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "host-gateway")
}

func TestValidate_ContainerDNSAcceptsValid(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.dns]\nservers = [\"10.0.0.53\", \"1.1.1.1\"]\nsearch = [\"corp.example.com\"]\n"
	require.NoError(t, loadWS(t, body))
}

func TestValidate_ContainerDNSRejectsBadServer(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.dns]\nservers = [\"not-an-ip\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "valid IPv4/IPv6")
}

func TestValidate_ContainerDNSRejectsDuplicate(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.dns]\nservers = [\"1.1.1.1\", \"1.1.1.1\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

func TestValidate_ContainerSysctlsAcceptsIntAndString(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.sysctls]\n\"vm.max_map_count\" = 262144\n\"kernel.shmmax\" = \"68719476736\"\n"
	require.NoError(t, loadWS(t, body))
}

func TestValidate_ContainerSysctlsRejectsBadKey(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.sysctls]\n\"BadKey\" = 1\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "sysctl key does not match")
}

func TestValidate_ContainerSysctlsRejectsBoolValue(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.sysctls]\n\"vm.swappiness\" = true\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be int or string")
}

func TestValidate_ContainerCapabilitiesAcceptsValid(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.capabilities]\nadd = [\"SYS_PTRACE\"]\ndrop = [\"AUDIT_WRITE\"]\n"
	require.NoError(t, loadWS(t, body))
}

func TestValidate_ContainerCapabilitiesRejectsLowercase(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.capabilities]\nadd = [\"sys_ptrace\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "capability does not match")
}

func TestValidate_ContainerCapabilitiesRejectsAddDropConflict(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.capabilities]\nadd = [\"NET_ADMIN\"]\ndrop = [\"NET_ADMIN\"]\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "appears in both add and drop")
}

func TestValidate_ContainerSecurityOptAcceptsValid(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.security_opt]\nseccomp = \"unconfined\"\nno_new_privileges = true\n"
	require.NoError(t, loadWS(t, body))
}

func TestValidate_ContainerSecurityOptRejectsEmptySeccomp(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.security_opt]\nseccomp = \"\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "seccomp must not be empty")
}

func TestValidate_ContainerSkelAcceptsValid(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[container.skel]]\nsource = \".cocoon/skel/.editorconfig\"\ntarget = \".editorconfig\"\n"
	require.NoError(t, loadWS(t, body))
}

func TestValidate_ContainerSkelRejectsAbsoluteSource(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[container.skel]]\nsource = \"/etc/passwd\"\ntarget = \".passwd\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no leading /")
}

func TestValidate_ContainerSkelRejectsTrailingSlashTarget(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[container.skel]]\nsource = \"a\"\ntarget = \".cfg/\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "files only")
}

func TestValidate_ContainerSkelRejectsDotDot(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[container.skel]]\nsource = \"a/../b\"\ntarget = \".x\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "`..`")
}

func TestValidate_ContainerSkelRejectsLeadingDash(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[container.skel]]\nsource = \"-from=builder/x\"\ntarget = \".x\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Dockerfile flag")
}

func TestValidate_ContainerSkelRejectsWhitespace(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[container.skel]]\nsource = \"a b\"\ntarget = \".x\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "whitespace or control")
}

func TestValidate_ContainerHostsRejectsUnderscore(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.hosts]\n\"db_local\" = \"127.0.0.1\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "hostname does not match")
}

func TestValidate_ContainerHostsRejectsDoubleDot(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.hosts]\n\"db..local\" = \"127.0.0.1\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "hostname does not match")
}

func TestValidate_ContainerSkelRejectsDuplicateTarget(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[container.skel]]\nsource = \"a\"\ntarget = \".x\"\n" +
		"\n[[container.skel]]\nsource = \"b\"\ntarget = \".x\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicates entry")
}

func TestValidate_AptMirrorAcceptsHTTP(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[apt.mirror]\nurl = \"http://jp.archive.ubuntu.com/ubuntu/\"\n"
	require.NoError(t, loadWS(t, body))
}

func TestValidate_AptMirrorRejectsBadScheme(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[apt.mirror]\nurl = \"ftp://example.com\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must start with http")
}

func TestValidate_AptMirrorRejectsSedDelimiter(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{
		"http://example.com/x|y",
		"http://example.com/x'y",
		"http://example.com/x&y",
		"http://example.com/x\\y",
		"http://example.com/x y",
		"http://example.com/x\ty",
		"http://example.com/x\ny",
	} {
		bad := bad
		t.Run(bad, func(t *testing.T) {
			t.Parallel()
			body := minimalWorkspace() +
				"\n[apt.mirror]\nurl = " + tomlString(bad) + "\n"
			err := loadWS(t, body)
			require.Error(t, err, "expected %q to be rejected", bad)
			require.Contains(t, err.Error(), "url must not contain")
		})
	}
}

// tomlString escapes s for embedding inside a TOML basic-string literal so
// the parser can round-trip control characters that we want to send through
// validation.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func TestValidate_AptProxyRejectsBadScheme(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[apt.proxy]\nhttp = \"socks5://x\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must start with http")
}

func TestValidate_AptSourcesAcceptsValid(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[apt.sources]]\nname = \"fish-stable\"\nsuite = \"noble\"\ncomponents = [\"main\"]\n" +
		"url = \"https://example.com/repo/\"\nkey_url = \"https://example.com/repo/key.gpg\"\n"
	require.NoError(t, loadWS(t, body))
}

func TestValidate_AptSourcesRejectsBadName(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[apt.sources]]\nname = \"BadName\"\nsuite = \"noble\"\ncomponents = [\"main\"]\n" +
		"url = \"https://example.com/repo/\"\nkey_url = \"https://example.com/repo/key.gpg\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "name does not match")
}

func TestValidate_AptSourcesRejectsEmptyComponents(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[apt.sources]]\nname = \"foo\"\nsuite = \"noble\"\ncomponents = []\n" +
		"url = \"https://example.com/repo/\"\nkey_url = \"https://example.com/repo/key.gpg\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "components must not be empty")
}

func TestValidate_SkelEmptySource(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[container.skel]]\nsource = \"\"\ntarget = \".cfg\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not be empty")
}

func TestValidate_SkelTilde(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[container.skel]]\nsource = \"~/foo\"\ntarget = \".cfg\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not start with ~")
}

func TestValidate_SkelColon(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[container.skel]]\nsource = \"a:b\"\ntarget = \".cfg\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not contain `:`")
}

func TestValidate_SkelWhitespace(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[container.skel]]\nsource = \"a b\"\ntarget = \".cfg\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "whitespace")
}

func TestValidate_LocaleInvalid(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[locale]\nlang = \"NotALocale\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "lang does not match")
}

func TestValidate_LocaleAccepts(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[locale]\nlang = \"en_US.UTF-8\"\n"
	require.NoError(t, loadWS(t, body))
}

func TestValidate_GitIdentityInvalidEmail(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[git]\nuser_email = \"not an email\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "user_email does not match")
}

func TestValidate_GitIdentityAccepts(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[git]\nuser_email = \"dev@example.com\"\n"
	require.NoError(t, loadWS(t, body))
}

func TestValidate_SecurityOptSeccompEmpty(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.security_opt]\nseccomp = \"\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "seccomp must not be empty")
}

func TestValidate_SecurityOptApparmorEmpty(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.security_opt]\napparmor = \"\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "apparmor must not be empty")
}

func TestValidate_AptProxyHTTPSBadScheme(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[apt.proxy]\nhttps = \"file:///etc\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "https must start with http")
}

func TestValidate_MountEmptySource(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[mounts]]\nsource = \"\"\ntarget = \"/abs/path\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "source must not be empty")
}

func TestValidate_MountRelativeTarget(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[mounts]]\nsource = \"./local/path\"\ntarget = \"relative/path\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "target must be an absolute path")
}

func TestValidate_MountAccepts(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[mounts]]\nsource = \"./local\"\ntarget = \"/etc/profile\"\n"
	require.NoError(t, loadWS(t, body))
}

func TestValidate_AptSourcesBadComponent(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[apt.sources]]\nname = \"foo\"\nsuite = \"noble\"\ncomponents = [\"BAD comp\"]\n" +
		"url = \"https://example.com/repo/\"\nkey_url = \"https://example.com/repo/key.gpg\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "component does not match")
}

func TestValidate_AptSourcesBadURL(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[apt.sources]]\nname = \"foo\"\nsuite = \"noble\"\ncomponents = [\"main\"]\n" +
		"url = \"ftp://bad/url\"\nkey_url = \"https://example.com/repo/key.gpg\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "url must start with http")
}

func TestValidate_AptSourcesBadKeyURL(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[apt.sources]]\nname = \"foo\"\nsuite = \"noble\"\ncomponents = [\"main\"]\n" +
		"url = \"https://example.com/repo/\"\nkey_url = \"ftp://bad\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "key_url must start with http")
}

func TestValidate_AptSourcesBadArch(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[apt.sources]]\nname = \"foo\"\nsuite = \"noble\"\ncomponents = [\"main\"]\n" +
		"url = \"https://example.com/repo/\"\nkey_url = \"https://example.com/repo/key.gpg\"\n" +
		"arch = \"riscv-32\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "arch must be")
}

func TestValidate_AptSourcesDuplicateNames(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[apt.sources]]\nname = \"foo\"\nsuite = \"noble\"\ncomponents = [\"main\"]\n" +
		"url = \"https://example.com/repo/\"\nkey_url = \"https://example.com/repo/key.gpg\"\n" +
		"\n[[apt.sources]]\nname = \"foo\"\nsuite = \"noble\"\ncomponents = [\"main\"]\n" +
		"url = \"https://example.com/repo/\"\nkey_url = \"https://example.com/repo/key.gpg\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicates entry")
}

func TestValidate_PluginsBadIDChar(t *testing.T) {
	t.Parallel()
	body := strings.ReplaceAll(minimalWorkspace(), "enable = []", `enable = ["BAD/id"]`)
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "plugin id does not match")
}

func TestValidate_PluginVersionPinEmpty(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[plugins.versions.go]\npin = \"\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pin must not be empty")
}

func TestValidate_PluginVersionBadChecksum(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[plugins.versions.go]\npin = \"1.23.4\"\nchecksum_amd64 = \"NOT-A-HEX\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "checksum_amd64")
}

func TestValidate_PortsOutOfRange(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[ports]\nforward = [70000]\n"
	err := loadWS(t, body)
	require.Error(t, err)
}

func TestValidate_SidecarRestartInvalid(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[services.app]\nimage = \"nginx\"\nrestart = \"sometimes\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
}

func TestValidate_SidecarRestartAccepted(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"no", "always", "on-failure", "unless-stopped"} {
		body := minimalWorkspace() +
			"\n[services.app]\nimage = \"nginx\"\nrestart = \"" + v + "\"\n"
		if err := loadWS(t, body); err != nil {
			t.Errorf("restart=%q err = %v", v, err)
		}
	}
}

func TestValidate_SidecarMountInvalid(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[services.app]\nimage = \"nginx\"\n" +
		"\n[[services.app.mounts]]\nsource = \"\"\ntarget = \"relative/path\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "source must not be empty")
}
