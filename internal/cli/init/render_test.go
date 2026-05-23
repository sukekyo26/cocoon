//nolint:testpackage // exercises the unexported renderWorkspaceToml renderer.
package initcli

import (
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/i18n"
)

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
	"# [plugins.methods]",
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

// TestRenderWorkspaceToml_ImagePathFix_PerImage pins the auto-comment +
// [container.shell.env] block emitted for every language image cocoon
// gates the prompt on. Catches accidental key renames (NPM_CONFIG_PREFIX,
// CARGO_INSTALL_ROOT) and value drift before the snapshot test would.
func TestRenderWorkspaceToml_ImagePathFix_PerImage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		image string
		// substrings that must appear in the rendered output
		wantContains []string
		// substrings that must NOT appear (sanity-checks the safe
		// CARGO_INSTALL_ROOT spelling does not flip back to CARGO_HOME)
		wantAbsent []string
	}{
		{
			image: "node",
			wantContains: []string{
				"on the node image.",
				"for `npm install -g <pkg>`",
				"[container.shell.env]\n" +
					"NPM_CONFIG_PREFIX = \"$HOME/.npm-global\"\n" +
					"PATH = \"$HOME/.npm-global/bin:$PATH\"\n",
			},
		},
		{
			image: "python",
			wantContains: []string{
				"on the python image.",
				"for `pip install --user <pkg>`",
				"[container.shell.env]\n" +
					"PATH = \"$HOME/.local/bin:$PATH\"\n",
			},
		},
		{
			image: "golang",
			wantContains: []string{
				"on the golang image.",
				"for `go install <pkg>@latest`",
				"[container.shell.env]\n" +
					"PATH = \"$HOME/go/bin:$PATH\"\n",
			},
		},
		{
			image: "rust",
			wantContains: []string{
				"on the rust image.",
				"for `cargo install <pkg>`",
				"[container.shell.env]\n" +
					"CARGO_INSTALL_ROOT = \"$HOME/.cargo\"\n" +
					"PATH = \"$HOME/.cargo/bin:$PATH\"\n",
			},
			wantAbsent: []string{
				// CARGO_HOME would clobber rustup state — the safe
				// spelling stays CARGO_INSTALL_ROOT.
				"CARGO_HOME =",
			},
		},
		{
			image: "denoland/deno",
			wantContains: []string{
				"on the denoland/deno image.",
				"for `deno install <script>`",
				"[container.shell.env]\n" +
					"PATH = \"$HOME/.deno/bin:$PATH\"\n",
			},
		},
	}
	cat := i18n.New(i18n.LangEN)
	for _, tc := range cases {
		t.Run(tc.image, func(t *testing.T) {
			t.Parallel()
			got := renderWorkspaceToml(containerSpec{
				ServiceName: "svc", Username: "dev",
				Image: tc.image, ImageVersion: "x",
				Shell: "bash", MountRoot: ".", Devcontainer: true,
				ImagePathFix: true,
			}, cat)
			for _, want := range tc.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("%s: output missing %q\n--- got ---\n%s", tc.image, want, got)
				}
			}
			for _, banned := range tc.wantAbsent {
				if strings.Contains(got, banned) {
					t.Errorf("%s: output must not contain %q", tc.image, banned)
				}
			}
		})
	}
}

// TestRenderWorkspaceToml_ImagePathFix_OffEmitsNothing pins that the
// auto-comment + [container.shell.env] block is silent when ImagePathFix
// is false even on an image that would otherwise trigger the prompt.
// Mirror-tests the --no-image-path-fix opt-out path.
func TestRenderWorkspaceToml_ImagePathFix_OffEmitsNothing(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", Image: "node", ImageVersion: "x",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
		ImagePathFix: false,
	}, cat)
	// "NPM_CONFIG_PREFIX" alone appears in the [container.shell] header
	// comment block as a worked example, so guard against the
	// load-bearing signals only: the subsection header and the actual
	// value we would emit.
	for _, banned := range []string{
		"[container.shell.env]",
		`NPM_CONFIG_PREFIX = "$HOME/.npm-global"`,
		"Added by `cocoon init`",
	} {
		if strings.Contains(got, banned) {
			t.Errorf("ImagePathFix=false should not emit %q\n--- got ---\n%s", banned, got)
		}
	}
}

// TestRenderWorkspaceToml_ImagePathFix_NonApplicableImageIgnored pins
// that ImagePathFix=true is a no-op when the image has no fix entry —
// defends against a future caller forgetting the imagePathFixApplies
// gate.
func TestRenderWorkspaceToml_ImagePathFix_NonApplicableImageIgnored(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "26.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
		ImagePathFix: true,
	}, cat)
	if strings.Contains(got, "[container.shell.env]") {
		t.Errorf("ImagePathFix=true on ubuntu must not emit [container.shell.env]\n--- got ---\n%s", got)
	}
}

// TestRenderWorkspaceToml_ImagePathFix_LocalizedJA pins the Japanese
// version of the auto-comment so the doc-comment contract holds across
// locales.
func TestRenderWorkspaceToml_ImagePathFix_LocalizedJA(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangJA)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", Image: "node", ImageVersion: "x",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
		ImagePathFix: true,
	}, cat)
	for _, want := range []string{
		"node イメージで user-local ツール",
		"再び sudo が必要",
		"[container.shell.env]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("JA output missing %q\n--- got ---\n%s", want, got)
		}
	}
}
