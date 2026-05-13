package plugin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

// TestPluginContracts replaces the per-plugin Bash tests previously living
// under tests/unit/plugins/. For every plugin in plugins/<id>/ it asserts:
//
//   - metadata fields (Name, Default, Install.RequiresRoot, Version.VersionCapable)
//   - first auto-derived volume name (when the plugin declares volumes)
//   - the plugin's install.sh content contains required substrings
//     (security knobs like tlsv1.2 / retry / sha256sum / etc.)
//   - the plugin's install.sh content does NOT contain forbidden substrings
//     (legacy {{...}} placeholders, brittle deps like jq / api.github.com)
//
// Adding a new plugin only requires adding a row here.
func TestPluginContracts(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	pluginsDir := filepath.Join(repoRoot, "internal", "plugin", "catalog")

	type spec struct {
		id             string
		name           string
		requiresRoot   bool
		versionCapable bool
		firstVolume    string // empty = no volume to assert
		mustContain    []string
		mustNotContain []string
	}

	noPlaceholders := []string{
		"{{VERSION}}", "{{FETCH}}", "{{CHECKSUM_AMD64}}", "{{CHECKSUM_ARM64}}",
	}
	noApiNoJq := []string{"api.github.com", "| jq "}

	specs := []spec{
		{
			id: "aws-cli", name: "AWS CLI v2",
			requiresRoot: false, firstVolume: "aws",
			mustContain: []string{"AWS", "retry 3", "tlsv1.2"},
		},
		{
			id: "bun", name: "bun",
			requiresRoot: false, versionCapable: true, firstVolume: "bun",
			mustContain: []string{
				"bun", "BUN_INSTALL", "PATH", "tlsv1.2", "retry 3",
				"bun.sh/install", "bun.com/install",
				`"$RC_FILE"`, "RC_SYNTAX", "set -gx",
			},
			mustNotContain: []string{"{{FETCH}}", "{{VERSION}}", `>> ~/.bashrc`},
		},
		{
			id: "aws-sam-cli", name: "AWS SAM CLI",
			requiresRoot: false,
			mustContain:  []string{"SAM", "retry 3", "dpkg --print-architecture", "tlsv1.2"},
		},
		{
			id: "claude-code", name: "Claude Code",
			requiresRoot: false, firstVolume: "claude",
			mustContain: []string{"Claude", "retry 3", "curl -fsSL"},
		},
		{
			id: "copilot-cli", name: "GitHub Copilot CLI",
			requiresRoot: false, versionCapable: true, firstVolume: "copilot",
			mustContain: []string{
				"Copilot", "curl -fsSL", "retry 3",
				"gh.io/copilot-install", `PREFIX="$HOME/.local"`,
			},
			mustNotContain: []string{"{{FETCH}}", "{{VERSION}}"},
		},
		{
			id: "docker-cli", name: "Docker CLI",
			requiresRoot: true,
			mustContain:  []string{"Docker", "retry 3"},
		},
		{
			id: "github-cli", name: "GitHub CLI",
			requiresRoot: true, firstVolume: "config",
			mustContain: []string{"GitHub", "retry 3"},
		},
		{
			id: "go", name: "Go",
			requiresRoot: true, versionCapable: true, firstVolume: "go",
			mustContain: []string{"go.dev", "retry 3", "GOPATH", "tlsv1.2"},
		},
		{
			id: "google-chrome", name: "Google Chrome",
			requiresRoot: true,
			mustContain:  []string{"google-chrome-stable_current_amd64.deb", "retry 3", "tlsv1.2"},
		},
		{
			id: "lazygit", name: "lazygit",
			requiresRoot: true, versionCapable: true,
			mustContain:    []string{"lazygit", "sha256sum -c -", "tlsv1.2", "retry 3"},
			mustNotContain: append(append([]string{}, noPlaceholders...), noApiNoJq...),
		},
		{
			id: "mise", name: "mise",
			requiresRoot: false, versionCapable: true, firstVolume: "mise",
			mustContain: []string{
				"mise", "PATH", "tlsv1.2", "retry 3", "mise.run",
				"MISE_VERSION", "MISE_DATA_DIR", `"$RC_FILE"`, "LOGIN_SHELL", "mise activate",
			},
			mustNotContain: []string{"{{FETCH}}", "{{VERSION}}", `>> ~/.bashrc`},
		},
		{
			id: "nerd-fonts", name: "Nerd Fonts",
			requiresRoot: false, firstVolume: "fonts",
			mustContain: []string{"Meslo", "retry 3", "fc-cache", "tlsv1.2", ".fonts"},
		},
		{
			id: "node", name: "Node.js",
			requiresRoot: true, versionCapable: true, firstVolume: "npm",
			mustContain: []string{
				"nodejs.org/dist", "NODE_ARCH", "sha256sum -c -",
				"tlsv1.2", "retry 3", "dpkg --print-architecture", "tar",
			},
			mustNotContain: append(append([]string{}, noPlaceholders...), noApiNoJq...),
		},
		{
			id: "deno", name: "Deno",
			requiresRoot: true, versionCapable: true, firstVolume: "deno",
			mustContain: []string{
				"deno", "DENO_ARCH", "github.com/denoland/deno", "sha256sum -c -",
				"tlsv1.2", "retry 3", "dpkg --print-architecture", "unzip",
			},
			mustNotContain: append(append([]string{}, noPlaceholders...), "api.github.com", "| jq "),
		},
		{
			id: "opentofu", name: "OpenTofu",
			requiresRoot: true, versionCapable: true,
			mustContain: []string{
				"opentofu", "tofu", "sha256sum -c -", "tlsv1.2", "retry 3",
				"github.com/opentofu/opentofu",
			},
			mustNotContain: append(append([]string{}, noPlaceholders...), "api.github.com", "| jq "),
		},
		{
			id: "proto", name: "proto",
			requiresRoot: false, versionCapable: true, firstVolume: "proto",
			mustContain: []string{
				"proto", "PROTO_HOME", "PATH",
				"tlsv1.2", "retry 3", "moonrepo.dev/install/proto.sh",
				`"$RC_FILE"`, "RC_SYNTAX", "set -gx",
			},
			mustNotContain: []string{"{{FETCH}}", "{{VERSION}}", `>> ~/.bashrc`},
		},
		{
			id: "rust", name: "Rust",
			requiresRoot: false, firstVolume: "cargo",
			mustContain: []string{
				"rustup", "retry 3", "PATH", "tlsv1.2",
				`"$RC_FILE"`, "RC_SYNTAX", "env.fish",
			},
			mustNotContain: []string{`>> ~/.bashrc`},
		},
		{
			id: "kubectl", name: "kubectl",
			requiresRoot: true, versionCapable: true,
			mustContain: []string{
				"kubectl", "dl.k8s.io", "sha256sum -c -",
				"tlsv1.2", "retry 3", "dpkg --print-architecture",
			},
			mustNotContain: append(append([]string{}, noPlaceholders...), noApiNoJq...),
		},
		{
			id: "helm", name: "Helm",
			requiresRoot: true, versionCapable: true,
			mustContain: []string{
				"helm", "get.helm.sh", "sha256sum -c -",
				"tlsv1.2", "retry 3", "dpkg --print-architecture", "tar",
			},
			mustNotContain: append(append([]string{}, noPlaceholders...), noApiNoJq...),
		},
		{
			id: "shellcheck", name: "ShellCheck",
			requiresRoot: true, versionCapable: true,
			mustContain: []string{
				"shellcheck", "github.com/koalaman/shellcheck", "sha256sum -c -",
				"tlsv1.2", "retry 3", "dpkg --print-architecture", "tar -xJ",
			},
			mustNotContain: append(append([]string{}, noPlaceholders...), noApiNoJq...),
		},
		{
			id: "shfmt", name: "shfmt",
			requiresRoot: true, versionCapable: true,
			mustContain: []string{
				"shfmt", "github.com/mvdan/sh", "sha256sum -c -",
				"tlsv1.2", "retry 3", "dpkg --print-architecture",
			},
			mustNotContain: append(append([]string{}, noPlaceholders...), noApiNoJq...),
		},
		{
			id: "terraform", name: "Terraform",
			requiresRoot: true, versionCapable: true,
			mustContain: []string{
				"terraform", "releases.hashicorp.com", "sha256sum -c -",
				"tlsv1.2", "retry 3", "dpkg --print-architecture", "unzip",
			},
			mustNotContain: append(append([]string{}, noPlaceholders...), "api.github.com", "| jq "),
		},
		{
			id: "starship", name: "Starship",
			requiresRoot: true, versionCapable: true, firstVolume: "config",
			mustContain: []string{
				"starship", "sha256sum", "tlsv1.2", "retry 3",
				`"$RC_FILE"`, "LOGIN_SHELL", "starship init",
			},
			mustNotContain: append(append([]string{}, noApiNoJq...), `>> ~/.bashrc`),
		},
		{
			id: "uv", name: "uv",
			requiresRoot: false, versionCapable: true,
			mustContain: []string{
				"uv", "PATH", "tlsv1.2", "retry 3", "astral.sh/uv",
				`"$RC_FILE"`, "RC_SYNTAX", "set -gx",
			},
			mustNotContain: []string{"{{FETCH}}", "{{VERSION}}", `>> ~/.bashrc`},
		},
		{
			id:           "zig", // metadata.name not asserted (kept loose to mirror old test).
			requiresRoot: true, versionCapable: true,
			mustContain:    []string{"ziglang.org", "retry 3", "sha256sum -c -"},
			mustNotContain: noPlaceholders,
		},
	}

	// Sanity: every plugins/<id>/plugin.toml must have a contract row.
	covered := make(map[string]struct{}, len(specs))
	for _, s := range specs {
		covered[s.id] = struct{}{}
	}
	entries, err := filepath.Glob(filepath.Join(pluginsDir, "*", "plugin.toml"))
	if err != nil {
		t.Fatalf("glob plugins: %v", err)
	}
	for _, e := range entries {
		id := filepath.Base(filepath.Dir(e))
		if _, ok := covered[id]; !ok {
			t.Errorf("plugin %q has no contract spec in TestPluginContracts", id)
		}
	}

	for _, s := range specs {
		s := s
		t.Run(s.id, func(t *testing.T) {
			t.Parallel()

			pluginToml := filepath.Join(pluginsDir, s.id, "plugin.toml")
			p, err := plugin.Load(pluginToml)
			if err != nil {
				t.Fatalf("load plugin: %v", err)
			}

			if s.name != "" && p.Metadata.Name != s.name {
				t.Errorf("metadata.name = %q, want %q", p.Metadata.Name, s.name)
			}
			if p.Install.RequiresRoot != s.requiresRoot {
				t.Errorf("install.requires_root = %v, want %v", p.Install.RequiresRoot, s.requiresRoot)
			}
			if p.Version.VersionCapable != s.versionCapable {
				t.Errorf("version.version_capable = %v, want %v", p.Version.VersionCapable, s.versionCapable)
			}
			if s.firstVolume != "" {
				vols := plugin.GetVolumes([]string{s.id}, map[string]*plugin.Plugin{s.id: p})
				if len(vols) == 0 {
					t.Fatalf("no derived volumes; want first=%q", s.firstVolume)
				}
				if vols[0].VolumeName != s.firstVolume {
					t.Errorf("first volume = %q, want %q", vols[0].VolumeName, s.firstVolume)
				}
			}

			installSh, err := os.ReadFile(filepath.Join(pluginsDir, s.id, "install.sh"))
			if err != nil {
				t.Fatalf("read install.sh: %v", err)
			}
			content := string(installSh)
			// Some plugins also ship install_user.sh; concatenate so contract
			// substrings can live in either file. plugin.toml is also folded
			// in so checks like "PATH"/"GOPATH" set under [install.env] match.
			if extra, err := os.ReadFile(filepath.Join(pluginsDir, s.id, "install_user.sh")); err == nil {
				content += "\n" + string(extra)
			}
			if extra, err := os.ReadFile(pluginToml); err == nil {
				content += "\n" + string(extra)
			}

			for _, want := range s.mustContain {
				if !strings.Contains(content, want) {
					t.Errorf("plugin %s install scripts missing %q", s.id, want)
				}
			}
			for _, bad := range s.mustNotContain {
				if strings.Contains(content, bad) {
					t.Errorf("plugin %s install scripts must not contain %q", s.id, bad)
				}
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}
