package setup_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	generatecli "github.com/sukekyo26/cocoon/internal/cli/generate"
	"github.com/sukekyo26/cocoon/internal/repositories"
	"github.com/sukekyo26/cocoon/internal/setup"
)

// TestRun_Scenarios is the Go replacement for the bash integration suite at
// tests/integration/test_setup_docker.sh. It exercises setup.Run end-to-end
// against a temp workspace seeded with a symlink to the real plugins/ tree
// and verifies the same behaviours that the bash tests covered (regenerate,
// --init --yes, partial workspace.toml, [devcontainer] preservation).
func TestRun_Scenarios(t *testing.T) {
	t.Parallel()

	repoRoot := repoRootFromCWD(t)
	pluginsDir := filepath.Join(repoRoot, "plugins")

	type expect struct {
		path           string
		mustContain    []string
		mustNotContain []string
	}

	type tc struct {
		name        string
		setupTOML   string // workspace.toml body to seed (empty = none)
		opts        setup.Options
		assertFiles []expect
		// stdoutContains lets a scenario assert that printResult emitted a
		// specific i18n key + arg combination. Substrings are matched against
		// the buffered stdout, with noopCatalog rendering messages as
		// "key: arg1, arg2, ..." so the assertion is resilient to translation
		// edits while still catching missed keys / wrong args.
		stdoutContains []string
		// expectRunErr asserts that setup.Run returns a non-nil error. Used
		// for migration / validation scenarios where the run is meant to fail
		// before any artefact is generated. runErrContains lets the test pin
		// substrings of the returned error so a regression in error
		// messaging surfaces here.
		expectRunErr   bool
		runErrContains []string
	}

	cases := []tc{
		{
			name: "regenerate_from_existing_toml",
			setupTOML: `[container]
service_name = "setup-test"
username = "testuser"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = ["docker-cli", "github-cli"]

[ports]
forward = ["8080:8080"]

[apt]
packages = []
`,
			assertFiles: []expect{
				{path: "Dockerfile", mustContain: []string{"Docker CLI", "GitHub CLI"}, mustNotContain: []string{"Install AWS CLI", "{{"}},
				{path: "docker-compose.yml", mustContain: []string{"setup-test"}},
				{path: ".devcontainer/devcontainer.json", mustContain: []string{"setup-test"}},
				{path: ".devcontainer/docker-compose.yml", mustContain: []string{"setup-test"}},
				{path: ".env", mustContain: []string{
					"COMPOSE_PROJECT_NAME=",
					"CONTAINER_SERVICE_NAME=setup-test",
					"USERNAME=testuser",
					"OS_IMAGE=ubuntu",
					"OS_VERSION=24.04",
				}, mustNotContain: []string{"FORWARD_PORT"}},
			},
		},
		{
			name: "regenerate_no_plugins",
			setupTOML: `[container]
service_name = "minimal"
username = "testuser"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = []

[apt]
packages = []
`,
			assertFiles: []expect{
				{path: "Dockerfile", mustNotContain: []string{
					"Install Docker CLI", "Install AWS CLI", "proto",
				}},
			},
		},
		{
			name:      "init_with_yes_flag_creates_defaults",
			setupTOML: "",
			opts:      setup.Options{ForceInit: true, AutoYes: true},
			assertFiles: []expect{
				{path: "workspace.toml", mustContain: []string{
					`service_name = "dev"`,
					`os = "ubuntu"`,
					`os_version = "24.04"`,
					`forward = []`,
					"[apt]",
					"packages = []",
					"[volumes]",
				}, mustNotContain: []string{"[vscode]"}},
				{path: "Dockerfile", mustContain: nil},
				{path: "docker-compose.yml", mustContain: nil},
				{path: ".devcontainer/devcontainer.json", mustContain: nil},
				{path: ".env", mustContain: []string{"COMPOSE_PROJECT_NAME="}},
			},
		},
		{
			name: "partial_toml_without_container_with_yes",
			setupTOML: `[plugins]
enable = ["docker-cli", "github-cli"]

[ports]
forward = ["8080:8080"]

[volumes]
deno = "/home/${USERNAME}/.deno"

[devcontainer.customizations.vscode]
extensions = ["eamodio.gitlens"]
`,
			opts: setup.Options{AutoYes: true},
			assertFiles: []expect{
				{path: "workspace.toml", mustContain: []string{
					"[container]",
					`service_name = "dev"`,
					"docker-cli", "github-cli",
					`forward = ["8080:8080"]`,
					"eamodio.gitlens",
					"deno",
				}},
				{path: "Dockerfile", mustContain: []string{"Docker CLI", "GitHub CLI"}},
				{path: "docker-compose.yml", mustContain: nil},
				{path: ".devcontainer/devcontainer.json", mustContain: []string{"eamodio.gitlens"}},
				{path: ".env", mustContain: []string{"USERNAME="}},
			},
		},
		{
			name: "ubuntu_26_04_propagates_through_generated_files",
			setupTOML: `[container]
service_name = "noble-test"
username = "testuser"
os = "ubuntu"
os_version = "26.04"

[plugins]
enable = []

[apt]
packages = []
`,
			assertFiles: []expect{
				{path: "workspace.toml", mustContain: []string{
					`os = "ubuntu"`,
					`os_version = "26.04"`,
				}},
				{path: ".env", mustContain: []string{
					"OS_IMAGE=ubuntu",
					"OS_VERSION=26.04",
				}},
				// Dockerfile must bake 26.04 as the OS_VERSION ARG default so a
				// build invoked without --build-arg OS_VERSION=... still resolves
				// to 26.04. The `FROM ${OS_IMAGE}:${OS_VERSION}` line alone would
				// pass even if the ARG default regressed; only the literal
				// `ARG OS_VERSION=26.04` (and OS_IMAGE=ubuntu) proves the
				// workspace.toml value reached the Dockerfile.
				{path: "Dockerfile", mustContain: []string{
					"ARG OS_IMAGE=ubuntu",
					"ARG OS_VERSION=26.04",
					"FROM ${OS_IMAGE}:${OS_VERSION}",
				}},
			},
			// printResult must surface the chosen OS+version in the setup
			// summary so users see what they got. Stdout was previously discarded,
			// which let a wrong i18n key or a missing log line slip through CI.
			stdoutContains: []string{
				"setup_result_os: ubuntu",
				"setup_result_os_version: 26.04",
			},
		},
		{
			name: "debian_12_propagates_through_generated_files",
			setupTOML: `[container]
service_name = "debian-test"
username = "testuser"
os = "debian"
os_version = "12"

[plugins]
enable = []

[apt]
packages = []
`,
			assertFiles: []expect{
				{path: "workspace.toml", mustContain: []string{
					`os = "debian"`,
					`os_version = "12"`,
				}},
				{path: ".env", mustContain: []string{
					"OS_IMAGE=debian",
					"OS_VERSION=12",
				}},
				// FROM line is the OS+version smoke; the apt mirror rewrite
				// block flips its target URL set per OS, so an
				// archive.ubuntu.com sed expression appearing in a Debian
				// build would silently break [apt.mirror] without any FROM
				// regression.
				{path: "Dockerfile", mustContain: []string{
					"ARG OS_IMAGE=debian",
					"ARG OS_VERSION=12",
					"FROM ${OS_IMAGE}:${OS_VERSION}",
				}},
			},
			stdoutContains: []string{
				"setup_result_os: debian",
				"setup_result_os_version: 12",
			},
		},
		{
			name: "legacy_ubuntu_version_is_rejected_with_migration_error",
			setupTOML: `[container]
service_name = "legacy"
username = "legacy"
ubuntu_version = "24.04"

[plugins]
enable = []

[apt]
packages = []
`,
			expectRunErr: true,
			runErrContains: []string{
				"ubuntu_version is no longer supported",
				`os = "ubuntu"`,
				`os_version = "24.04"`,
			},
		},
		{
			name: "force_init_yes_preserves_existing_os_version",
			setupTOML: `[container]
service_name = "old"
username = "olduser"
os = "ubuntu"
os_version = "22.04"

[plugins]
enable = []

[apt]
packages = []
`,
			opts: setup.Options{ForceInit: true, AutoYes: true},
			assertFiles: []expect{
				{path: "workspace.toml", mustContain: []string{
					`os = "ubuntu"`,
					`os_version = "22.04"`,
				}},
				{path: ".env", mustContain: []string{
					"OS_IMAGE=ubuntu",
					"OS_VERSION=22.04",
				}},
			},
		},
		{
			name: "devcontainer_preserved_on_force_init_yes",
			setupTOML: `[container]
service_name = "dc-init"
username = "testuser"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = ["docker-cli"]

[apt]
packages = []

[devcontainer]
remoteUser = "testuser"
`,
			opts: setup.Options{ForceInit: true, AutoYes: true},
			assertFiles: []expect{
				{path: "workspace.toml", mustContain: []string{
					"[devcontainer]",
					"remoteUser",
				}},
				{path: ".devcontainer/devcontainer.json", mustContain: []string{"remoteUser"}},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			work := t.TempDir()
			if c.setupTOML != "" {
				//nolint:gosec // workspace.toml is not a secret; user-readable mode mirrors production.
				if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(c.setupTOML), 0o644); err != nil {
					t.Fatalf("write workspace.toml: %v", err)
				}
			}

			var stdout bytes.Buffer
			opts := c.opts
			opts.WorkspaceDir = work
			opts.PluginsDir = pluginsDir
			opts.NoClone = true
			opts.Stdin = strings.NewReader("")
			opts.Stdout = &stdout
			opts.Stderr = io.Discard
			opts.Catalog = noopCatalog{}
			opts.Selector = nil // unused: AutoYes paths bypass the selector
			opts.Generator = generatorFunc(func(wsPath, pdir, outDir string, stderr io.Writer) error {
				var buf bytes.Buffer
				cmd := generatecli.NewCommand(&buf, stderr)
				cmd.SetArgs([]string{wsPath, pdir, outDir})
				return cmd.Execute()
			})
			opts.Cloner = noopCloner{}
			opts.GIDDetector = fixedGID{gid: 999}

			runErr := setup.Run(opts)
			if c.expectRunErr {
				if runErr == nil {
					t.Fatalf("Run: expected error, got nil")
				}
				msg := runErr.Error()
				for _, want := range c.runErrContains {
					if !strings.Contains(msg, want) {
						t.Errorf("Run error missing %q\n--- got ---\n%s", want, msg)
					}
				}
				return
			}
			if runErr != nil {
				t.Fatalf("Run: %v", runErr)
			}

			for _, e := range c.assertFiles {
				abs := filepath.Join(work, e.path)
				body, err := os.ReadFile(abs)
				if err != nil {
					t.Fatalf("read %s: %v", e.path, err)
				}
				s := string(body)
				for _, want := range e.mustContain {
					if !strings.Contains(s, want) {
						t.Errorf("%s missing %q\n--- got ---\n%s", e.path, want, s)
					}
				}
				for _, bad := range e.mustNotContain {
					if strings.Contains(s, bad) {
						t.Errorf("%s must not contain %q\n--- got ---\n%s", e.path, bad, s)
					}
				}
			}

			// .env permission must be 0600 when present.
			if st, err := os.Stat(filepath.Join(work, ".env")); err == nil {
				if st.Mode().Perm() != 0o600 {
					t.Errorf(".env perm = %o, want 600", st.Mode().Perm())
				}
			}

			out := stdout.String()
			for _, want := range c.stdoutContains {
				if !strings.Contains(out, want) {
					t.Errorf("stdout missing %q\n--- got ---\n%s", want, out)
				}
			}
		})
	}
}

// --- test doubles ---------------------------------------------------------

type noopCatalog struct{}

// Msg renders messages as "key: arg1, arg2, ..." (or just "key" when args is
// empty). Tests assert against this synthetic format instead of real i18n
// strings so translation edits do not cause spurious test failures, but
// missed keys / wrong args still surface as assertion misses.
func (noopCatalog) Msg(key string, args ...any) string {
	if len(args) == 0 {
		return key
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = fmt.Sprintf("%v", a)
	}
	return key + ": " + strings.Join(parts, ", ")
}

type generatorFunc func(wsPath, pluginsDir, outputDir string, stderr io.Writer) error

func (f generatorFunc) GenerateAll(wsPath, pluginsDir, outputDir string, stderr io.Writer) error {
	return f(wsPath, pluginsDir, outputDir, stderr)
}

type noopCloner struct{}

func (noopCloner) CloneAll(string, func(string, string)) (repositories.CloneSummary, error) {
	return repositories.CloneSummary{}, nil
}

type fixedGID struct{ gid int }

func (f fixedGID) Detect() (int, error) { return f.gid, nil }

func repoRootFromCWD(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// internal/setup -> repo root
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}
