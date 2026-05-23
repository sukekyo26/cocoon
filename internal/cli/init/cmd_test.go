//nolint:testpackage // exercises the cobra wiring of `cocoon init` end-to-end.
package initcli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
)

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

// TestRunInit_ImagePathFixConflict pins the mutual-exclusion check on
// --image-path-fix / --no-image-path-fix at the cobra wiring layer.
//
//nolint:paralleltest // t.Chdir
func TestRunInit_ImagePathFixConflict(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y",
		"--image", "node",
		"--image-path-fix", "--no-image-path-fix",
	})
	if err := cmd.Execute(); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("conflicting --image-path-fix flags should be ErrUsage, got %v", err)
	}
}

// TestRunInit_ImagePathFixFlagAgainstNonLanguageImage pins that the
// flag-against-ubuntu gate fires at the CLI layer, so a script that
// misses the image guard fails fast instead of writing an inconsistent
// workspace.toml.
//
//nolint:paralleltest // t.Chdir
func TestRunInit_ImagePathFixFlagAgainstNonLanguageImage(t *testing.T) {
	pinEnglish(t)
	work := t.TempDir()
	t.Chdir(work)
	cmd := NewCommand(io.Discard, io.Discard)
	cmd.SetArgs([]string{
		"--yes", "--service-name", "x", "--username", "y",
		"--image", "ubuntu", "--image-version", "22.04",
		"--image-path-fix",
	})
	if err := cmd.Execute(); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("--image=ubuntu --image-path-fix should be ErrUsage, got %v", err)
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
