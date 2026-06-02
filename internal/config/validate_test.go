package config_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/config"
)

// tomlQuote returns s as a TOML basic-string literal (`"..."`) suitable
// for embedding adversarial test inputs into a workspace.toml body. The
// callers in this file only feed characters whose Go-style escape from
// strconv.Quote happens to overlap with TOML basic-string escapes:
// \n \t \r \\ \" plus printable ASCII passed verbatim. Callers must
// not pass arbitrary bytes here — strconv.Quote also emits \a, \v, \f
// and \xNN hex escapes for other control / non-ASCII bytes, none of
// which TOML basic strings accept, and the resulting body would fail to
// parse before validate even runs.
func tomlQuote(s string) string { return strconv.Quote(s) }

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
	var v *config.ValidationError
	require.ErrorAs(t, err, &v)
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
	var v *config.ValidationError
	require.ErrorAs(t, err, &v)
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
	var v *config.ValidationError
	require.ErrorAs(t, err, &v)
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
	var v *config.ValidationError
	require.ErrorAs(t, err, &v)
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
		// Assert the SupportedImageVersions row exists and is non-empty
		// *before* iterating it. A bare `for _, v := range nil` would
		// silently iterate zero times and let the test pass even when
		// SupportedImages and SupportedImageVersions go out of sync —
		// exactly the desync this test exists to catch.
		versions, hit := config.SupportedImageVersions[image]
		require.Truef(t, hit, "SupportedImageVersions has no row for image %q", image)
		require.NotEmptyf(t, versions, "SupportedImageVersions[%q] is empty", image)
		for _, version := range versions {
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
		{"golang", "1.26.4-bookworm"}, // hypothetical future patch
		{"golang", "1.99-bookworm"},   // hypothetical future minor
		{"node", "27-bookworm-slim"},  // hypothetical future major
		{"python", "3.15-slim-bookworm"},
		{"denoland/deno", "debian-2.7.99"},
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

// TestValidate_ImagePluginConflict exercises the cross-section check
// that rejects workspace.toml files combining a language-runtime base
// image with the matching cocoon plugin. The conflict pairs come from
// ImageProvidesPlugin (image="golang" ↔ plugin "go", image="rust" ↔
// plugin "rust", image="node" ↔ plugin "node", image="denoland/deno" ↔
// plugin "deno"); other combinations (image="python" + uv plugin,
// image="ubuntu" + go plugin) must remain accepted because they
// coexist cleanly.
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
			name:  "golang_image_plus_go_plugin",
			image: "golang", version: "1.26-bookworm",
			enable:      `["go"]`,
			wantErr:     true,
			mustContain: `image = "golang" already provides go`,
		},
		{
			name:  "rust_image_plus_rust_plugin",
			image: "rust", version: "1.95-bookworm",
			enable:      `["rust"]`,
			wantErr:     true,
			mustContain: `image = "rust" already provides rust`,
		},
		{
			name:  "node_image_plus_node_plugin",
			image: "node", version: "24-bookworm-slim",
			enable:      `["node"]`,
			wantErr:     true,
			mustContain: `image = "node" already provides node`,
		},
		{
			name:  "deno_image_plus_deno_plugin",
			image: "denoland/deno", version: "debian-2.7.14",
			enable:      `["deno"]`,
			wantErr:     true,
			mustContain: `image = "denoland/deno" already provides deno`,
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
			name:  "golang_image_alone_ok",
			image: "golang", version: "1.26-bookworm",
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

func TestValidate_ShellValueRejectsNewline(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		spec string
	}{
		{"env newline", `env = { EDITOR = "a\nb" }`},
		{"env carriage return", `env = { EDITOR = "a\rb" }`},
		{"alias newline", `aliases = { gs = "git status\nRUN echo pwned" }`},
		{"alias carriage return", `aliases = { gs = "a\rb" }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := minimalWorkspace() + "\n[container.shell]\n" + tc.spec + "\n"
			err := loadWS(t, body)
			require.Error(t, err)
			require.Contains(t, err.Error(), "unsafe character")
		})
	}
}

// TestValidate_ShellValueAcceptsExpansion pins that $-expansion and command
// substitution stay legal in shell env/alias values — only newlines are
// rejected — so the documented $HOME / $(cmd) usage keeps working.
func TestValidate_ShellValueAcceptsExpansion(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() + "\n[container.shell]\n" +
		`env = { NPM_CONFIG_PREFIX = "$HOME/.local" }` + "\n" +
		`aliases = { ll = "ls -la $(pwd)" }` + "\n"
	require.NoError(t, loadWS(t, body))
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

// TestValidate_HomeFilesRejectsShellMetacharacters table-drives the
// shell-injection guard: each [home_files].files entry is interpolated
// raw into the generated initializeCommand snippet, so the validator
// must reject any path segment that carries shell-special meaning.
// Covers the obvious vectors (command substitution, separators, pipes,
// redirections, globs, history expansion, quoting, escapes, whitespace,
// embedded newlines, tilde, brace and bracket expansion).
func TestValidate_HomeFilesRejectsShellMetacharacters(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		entry string
	}{
		{"dollar_command_subst", ".cfg/$(whoami).json"},
		{"backtick_command_subst", ".cfg/`whoami`.json"},
		{"semicolon", ".cfg/a;b.json"},
		{"ampersand", ".cfg/a&b.json"},
		{"pipe", ".cfg/a|b.json"},
		{"redirect_lt", ".cfg/a<b.json"},
		{"redirect_gt", ".cfg/a>b.json"},
		{"glob_star", ".cfg/*.json"},
		{"glob_question", ".cfg/a?.json"},
		{"bang_history", ".cfg/a!b.json"},
		{"bracket", ".cfg/[a].json"},
		{"brace", ".cfg/{a,b}.json"},
		{"double_quote", ".cfg/a\"b.json"},
		{"single_quote", ".cfg/a'b.json"},
		{"backslash", ".cfg/a\\b.json"},
		{"space", ".cfg/a b.json"},
		{"tab", ".cfg/a\tb.json"},
		{"newline", ".cfg/a\nb.json"},
		{"paren_open", ".cfg/(a).json"},
		{"paren_close", ".cfg/a).json"},
		{"tilde_mid", ".cfg/~user/foo.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := minimalWorkspace() + "\n[home_files]\nfiles = [" +
				tomlQuote(tc.entry) + "]\n"
			err := loadWS(t, body)
			require.Error(t, err, "entry %q should be rejected", tc.entry)
			require.Contains(t, err.Error(), "[A-Za-z0-9._/-]+")
		})
	}
}

// TestValidate_HomeFilesAcceptsBenignPaths pins the whitelist's positive
// side: typical config-file paths that the docs encourage must continue
// to validate, including nested dotdirs (.gemini/oauth_creds.json) and
// hyphen-bearing names (.local/share/foo-bar.json).
func TestValidate_HomeFilesAcceptsBenignPaths(t *testing.T) {
	t.Parallel()
	cases := []string{
		".gitconfig",
		".claude.json",
		".gemini/oauth_creds.json",
		".local/share/foo-bar.json",
		".config/git/config",
		"plain_file_no_dot",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			t.Parallel()
			body := minimalWorkspace() + "\n[home_files]\nfiles = [" +
				tomlQuote(p) + "]\n"
			require.NoError(t, loadWS(t, body), "entry %q should be accepted", p)
		})
	}
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

func TestValidate_ContainerCapabilitiesRejectsEntrypointRequiredDrop(t *testing.T) {
	t.Parallel()
	for _, cap := range []string{
		"ALL", "CHOWN", "SETUID", "SETGID",
		"CAP_CHOWN", "CAP_SETUID", "CAP_SETGID",
	} {
		t.Run(cap, func(t *testing.T) {
			t.Parallel()
			body := minimalWorkspace() +
				"\n[container.capabilities]\ndrop = [\"" + cap + "\"]\n"
			err := loadWS(t, body)
			require.Error(t, err)
			require.Contains(t, err.Error(), "cannot be dropped")
		})
	}
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

func TestValidate_MountTargetAcceptsUsernamePlaceholder(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[[mounts]]\nsource = \"./local\"\ntarget = \"/home/${USERNAME}/.ssh\"\n"
	require.NoError(t, loadWS(t, body))
}

func TestValidate_MountTargetRejectsUnsafeChars(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		target string
	}{
		{"double-quote", `/etc/foo"bar`},
		{"backtick", "/etc/foo`bar"},
		{"bare-dollar", "/etc/$HOME/x"},
		{"colon", "/etc/foo:bar"},
		{"newline", "/etc/foo\nRUN echo pwned"},
		{"space", "/etc/foo bar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := minimalWorkspace() +
				"\n[[mounts]]\nsource = \"./local\"\ntarget = " + tomlString(tc.target) + "\n"
			err := loadWS(t, body)
			require.Error(t, err)
			require.Contains(t, err.Error(), "target may contain only")
		})
	}
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

// TestValidate_PluginsMethodsAccepted pins the happy path: a
// well-formed [plugins.methods] table passes validation.
func TestValidate_PluginsMethodsAccepted(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[plugins.methods]\ncopilot-cli = \"binary\"\ngithub-cli = \"apt\"\n"
	require.NoError(t, loadWS(t, body))
}

// TestValidate_PluginsMethodsBadPluginID pins that a key in
// [plugins.methods] is validated against the plugin id regex.
func TestValidate_PluginsMethodsBadPluginID(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[plugins.methods]\n\"BAD_ID\" = \"binary\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "plugin id does not match")
}

// TestValidate_PluginsMethodsBadMethodName pins that a value in
// [plugins.methods] is validated against the method name regex.
func TestValidate_PluginsMethodsBadMethodName(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[plugins.methods]\ncopilot-cli = \"BAD.METHOD\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "method name does not match")
}

// TestValidate_PluginsMethodsEmptyValueRejected pins that empty
// strings are rejected rather than silently treated as "no override":
// the user almost certainly meant to remove the entry.
func TestValidate_PluginsMethodsEmptyValueRejected(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[plugins.methods]\ncopilot-cli = \"\"\n"
	err := loadWS(t, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "method name does not match")
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

// containerWorkspace returns a minimal workspace.toml with extra lines
// injected inside the [container] section, for exercising flat [container]
// fields (group_add / devices / ipc / gpus).
func containerWorkspace(extra string) string {
	return `
[container]
service_name = "dev"
username = "developer"
image = "ubuntu"
image_version = "24.04"
` + extra + `
[plugins]
enable = []
`
}

func TestValidate_ContainerGroupAddAcceptsNamesAndGIDs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		toml string
	}{
		{"name", `group_add = ["audio"]`},
		{"name-with-dash", `group_add = ["host-users"]`},
		{"name-trailing-dollar", `group_add = ["machine$"]`},
		{"numeric-gid", `group_add = ["992"]`},
		{"mixed", `group_add = ["audio", "992", "dialout"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, loadWS(t, containerWorkspace(tc.toml+"\n")))
		})
	}
}

func TestValidate_ContainerGroupAddRejectsBadEntries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, toml, want string
	}{
		{"uppercase", `group_add = ["Audio"]`, "must be a group name"},
		{"leading-digit-name", `group_add = ["9audio"]`, "must be a group name"},
		{"empty", `group_add = [""]`, "must not be empty"},
		{"duplicate", `group_add = ["audio", "audio"]`, "duplicate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := loadWS(t, containerWorkspace(tc.toml+"\n"))
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidate_ContainerDevicesAcceptsValid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, toml string
	}{
		{"two-part", `devices = ["/dev/dri:/dev/dri"]`},
		{"three-part-perms", `devices = ["/dev/sda:/dev/xvda:rwm"]`},
		{"multiple", `devices = ["/dev/dri:/dev/dri", "/dev/fuse:/dev/fuse:rw"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, loadWS(t, containerWorkspace(tc.toml+"\n")))
		})
	}
}

func TestValidate_ContainerDevicesRejectsBadEntries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, toml, want string
	}{
		{"relative-host", `devices = ["dev/dri:/dev/dri"]`, "host path must be absolute"},
		{"relative-container", `devices = ["/dev/dri:dev/dri"]`, "container path must be absolute"},
		{"missing-container", `devices = ["/dev/dri"]`, "must be HOST:CONTAINER"},
		{"too-many-parts", `devices = ["/dev/dri:/dev/dri:rwm:x"]`, "must be HOST:CONTAINER"},
		{"bad-perms", `devices = ["/dev/dri:/dev/dri:xyz"]`, "cgroup permissions"},
		{"duplicate", `devices = ["/dev/dri:/dev/dri", "/dev/dri:/dev/dri"]`, "duplicate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := loadWS(t, containerWorkspace(tc.toml+"\n"))
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidate_ContainerIPCAcceptsValid(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{
		"none", "host", "private", "shareable",
		"container:other", "container:my-db.1",
	} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, loadWS(t, containerWorkspace("ipc = "+strconv.Quote(mode)+"\n")))
		})
	}
}

// ipc = "service:<name>" resolves against generated service names: the main
// service and any defined [services.<name>] sidecar. A typo must fail here
// rather than at `docker compose` time.
func TestValidate_ContainerIPCServiceTargetResolvesAgainstServices(t *testing.T) {
	t.Parallel()
	withSidecar := func(ipc string) string {
		return containerWorkspace("ipc = "+strconv.Quote(ipc)+"\n") + `
[services.db]
image = "postgres:16"
`
	}
	t.Run("defined-sidecar", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, loadWS(t, withSidecar("service:db")))
	})
	t.Run("main-service", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, loadWS(t, withSidecar("service:dev")))
	})
	t.Run("undefined-service", func(t *testing.T) {
		t.Parallel()
		err := loadWS(t, withSidecar("service:typo"))
		require.Error(t, err)
		require.Contains(t, err.Error(), `references undefined service "typo"`)
	})
}

func TestValidate_ContainerIPCRejectsBadValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, value, want string
	}{
		{"bogus", "bogus", "ipc must be one of"},
		{"service-no-name", "service:", "requires a target name"},
		{"container-no-name", "container:", "requires a target name"},
		{"container-space", "container:bad name", "not a valid Docker container name"},
		{"container-newline", "container:bad\nname", "not a valid Docker container name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := loadWS(t, containerWorkspace("ipc = "+strconv.Quote(tc.value)+"\n"))
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidate_ContainerGpusAcceptsAll(t *testing.T) {
	t.Parallel()
	require.NoError(t, loadWS(t, containerWorkspace("gpus = \"all\"\n")))
}

func TestValidate_ContainerGpusRejectsNonAll(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"2", "none", "ALL"} {
		t.Run(v, func(t *testing.T) {
			t.Parallel()
			err := loadWS(t, containerWorkspace("gpus = "+strconv.Quote(v)+"\n"))
			require.Error(t, err)
			require.Contains(t, err.Error(), `gpus must be "all"`)
		})
	}
}

// TestAccumulator_ZeroValueUsable pins the documented contract that the
// exported Accumulator works without NewAccumulator: At/Add lazily
// allocate the shared slice instead of nil-panicking, and an error
// recorded through a child created by At surfaces on the parent.
func TestAccumulator_ZeroValueUsable(t *testing.T) {
	t.Parallel()

	var a config.Accumulator
	require.Nil(t, a.Errors(), "fresh zero-value Errors() should be nil")

	a.At("container").Add("bad", "service_name")
	a.Add("top-level problem")

	errs := a.Errors()
	require.Len(t, errs, 2, "child (via At) and parent writes must share one slice")
	require.Equal(t, "bad", errs[0].Message)
	require.Equal(t, []string{"container", "service_name"}, errs[0].Loc)
}

// TestValidate_SudoMode pins the [container.sudo].mode enum: only the two
// sudoers policies are accepted; "none" is NOT a sudo mode (disabling sudo is
// no_new_privileges), and the check is case-sensitive.
func TestValidate_SudoMode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{"nopasswd", "nopasswd", false},
		{"password", "password", false},
		{"none_is_not_a_sudo_mode", "none", true},
		{"bogus", "bogus", true},
		{"wrong_case", "Password", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := minimalWorkspace() + "\n[container.sudo]\nmode = " + tomlQuote(tc.mode) + "\n"
			err := loadWS(t, body)
			if !tc.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			var v *config.ValidationError
			require.ErrorAs(t, err, &v)
			require.Contains(t, v.Errors[0].Message, "mode must be")
		})
	}
}

// TestValidate_PasswordSudoVsNoNewPrivileges pins the cross-field rejection:
// password sudo and no_new_privileges are mutually exclusive (the latter would
// make the password unusable). The error is reported on container.sudo.mode.
func TestValidate_PasswordSudoVsNoNewPrivileges(t *testing.T) {
	t.Parallel()
	body := minimalWorkspace() +
		"\n[container.sudo]\nmode = \"password\"\n" +
		"\n[container.security_opt]\nno_new_privileges = true\n"
	err := loadWS(t, body)
	require.Error(t, err)
	var v *config.ValidationError
	require.ErrorAs(t, err, &v)
	var found bool
	for i := range v.Errors {
		loc := v.Errors[i].Loc
		if len(loc) > 0 && loc[len(loc)-1] == "mode" &&
			strings.Contains(v.Errors[i].Message, "cannot be combined") {
			found = true
		}
	}
	require.Truef(t, found, "expected a container.sudo.mode mutual-exclusion error, got: %v", v.Errors)
}
