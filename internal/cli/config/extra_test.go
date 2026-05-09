package configcli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configcli "github.com/sukekyo26/cocoon/internal/cli/config"
)

const dumpDcTOML = `
[container]
service_name = "dev"
username = "dev"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = []

[devcontainer]
image = "ubuntu:24.04"
remoteUser = "dev"
`

const noDcTOML = `
[container]
service_name = "dev"
username = "dev"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = []
`

func writeTempTOML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "workspace.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runCmd(args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := configcli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

func TestPrintUsage(t *testing.T) {
	t.Parallel()
	stdout, _, err := runCmd()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "wsd config") {
		t.Errorf("usage banner missing: %q", stdout.String())
	}
	stdout, _, err = runCmd("help")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "wsd config") {
		t.Errorf("help banner missing from stdout: %q", stdout.String())
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("bogus")
	if !errors.Is(err, configcli.ErrUsage) {
		t.Fatalf("err = %v, want configcli.ErrUsage", err)
	}
}

func TestCmdDumpDevcontainer_WithSection(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, dumpDcTOML)
	stdout, stderr, err := runCmd("dump-devcontainer", path)
	if err != nil {
		t.Fatalf("err = %v stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "[devcontainer]") {
		t.Errorf("missing [devcontainer] header: %q", out)
	}
	if !strings.Contains(out, "ubuntu:24.04") {
		t.Errorf("missing image value: %q", out)
	}
}

func TestCmdDumpDevcontainer_NoSection(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, noDcTOML)
	stdout, _, err := runCmd("dump-devcontainer", path)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty when [devcontainer] absent, got %q", stdout.String())
	}
}

func TestCmdDumpDevcontainer_MissingArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("dump-devcontainer")
	if !errors.Is(err, configcli.ErrUsage) {
		t.Fatalf("err = %v, want configcli.ErrUsage", err)
	}
}

func TestCmdDumpDevcontainer_InvalidFile(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("dump-devcontainer", "/nonexistent.toml")
	if !errors.Is(err, configcli.ErrFailure) {
		t.Fatalf("err = %v, want configcli.ErrFailure", err)
	}
}

func TestCmdValidatePlugins_OK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "myplugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pluginToml := `
[metadata]
name = "myplugin"
description = "test"
default = false

[install]
requires_root = false

[version]
version_capable = false
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(pluginToml), 0o600); err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // plugin install.sh fixture must be executable for the plugin contract
	if err := os.WriteFile(filepath.Join(pluginDir, "install.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runCmd("validate-plugins", dir)
	if err != nil {
		t.Fatalf("err = %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK:") {
		t.Errorf("missing OK message: %q", stdout.String())
	}
}

func TestCmdValidatePlugins_MissingInstallSh(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "myplugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pluginToml := `
[metadata]
name = "myplugin"
description = "test"
default = false

[install]
requires_root = false

[version]
version_capable = false
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(pluginToml), 0o600); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := runCmd("validate-plugins", dir)
	if !errors.Is(err, configcli.ErrFailure) {
		t.Fatalf("err = %v, want configcli.ErrFailure", err)
	}
	if !strings.Contains(stderr.String(), "install.sh") {
		t.Errorf("expected stderr to mention install.sh: %q", stderr.String())
	}
}

func TestCmdValidatePlugins_NotDirectory(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("validate-plugins", "/nonexistent/directory")
	if !errors.Is(err, configcli.ErrFailure) {
		t.Fatalf("err = %v, want configcli.ErrFailure", err)
	}
}

func TestCmdValidatePlugins_MissingArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("validate-plugins")
	if !errors.Is(err, configcli.ErrUsage) {
		t.Fatalf("err = %v, want configcli.ErrUsage", err)
	}
}

func TestCmdValidateWorkspace_OK(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, noDcTOML)
	stdout, stderr, err := runCmd("validate-workspace", path)
	if err != nil {
		t.Fatalf("err = %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK:") {
		t.Errorf("missing OK message: %q", stdout.String())
	}
}

func TestCmdValidateWorkspace_MissingFile(t *testing.T) {
	t.Parallel()
	_, stderr, err := runCmd("validate-workspace", filepath.Join(t.TempDir(), "missing.toml"))
	if !errors.Is(err, configcli.ErrFailure) {
		t.Fatalf("err = %v, want configcli.ErrFailure", err)
	}
	if !strings.Contains(stderr.String(), "ERROR:") {
		t.Errorf("expected ERROR prefix: %q", stderr.String())
	}
}

func TestCmdValidateWorkspace_ValidationError(t *testing.T) {
	t.Parallel()
	// Missing required [container] block produces a ValidationError that
	// flows through printValidationErrors.
	bad := `
[plugins]
enable = []
`
	path := writeTempTOML(t, bad)
	_, stderr, err := runCmd("validate-workspace", path)
	if !errors.Is(err, configcli.ErrFailure) {
		t.Fatalf("err = %v, want configcli.ErrFailure", err)
	}
	if !strings.Contains(stderr.String(), "ERROR:") {
		t.Errorf("expected printValidationErrors output: %q", stderr.String())
	}
}

func TestCmdValidateWorkspace_PluginMissingDir(t *testing.T) {
	t.Parallel()
	body := `
[container]
service_name = "dev"
username = "dev"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = ["bogus-plugin"]
`
	path := writeTempTOML(t, body)
	pluginsRoot := t.TempDir()
	_, stderr, err := runCmd("validate-workspace", path, pluginsRoot)
	if !errors.Is(err, configcli.ErrFailure) {
		t.Fatalf("err = %v, want configcli.ErrFailure", err)
	}
	if !strings.Contains(stderr.String(), "bogus-plugin") {
		t.Errorf("expected mention of bogus-plugin: %q", stderr.String())
	}
}

func TestCmdValidateWorkspace_MissingArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("validate-workspace")
	if !errors.Is(err, configcli.ErrUsage) {
		t.Fatalf("err = %v, want configcli.ErrUsage", err)
	}
}

const listFixtureTOML = `
[container]
service_name = "dev"
username = "dev"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = ["docker-cli", "github-cli"]

[apt]
packages = ["jq", "curl"]

[ports]
forward = ["3000:3000", "8080:8080", { target = 5432, published = 5432, host_ip = "127.0.0.1", protocol = "tcp" }]

[volumes]
home = "/home/${USERNAME}"
cache = "/cache"
`

func TestCmdList_Plugins(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, listFixtureTOML)
	stdout, _, err := runCmd("list", path, "plugins")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "docker-cli\n") || !strings.Contains(out, "github-cli\n") {
		t.Errorf("expected plugin names line-per-line: %q", out)
	}
}

func TestCmdList_ForwardPorts(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, listFixtureTOML)
	stdout, _, err := runCmd("list", path, "forward-ports")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := "3000:3000\n8080:8080\n127.0.0.1:5432:5432/tcp\n"
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("expected ports list %q, got %q", want, stdout.String())
	}
}

func TestCmdList_AptExtra(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, listFixtureTOML)
	stdout, _, err := runCmd("list", path, "apt-extra")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "jq\n") {
		t.Errorf("expected apt-extra: %q", stdout.String())
	}
}

func TestCmdList_DefaultForwardPort(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, noDcTOML) // no [ports] section
	stdout, _, err := runCmd("list", path, "forward-ports")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "3000:3000\n") {
		t.Errorf("expected default 3000:3000: %q", stdout.String())
	}
}

func TestCmdList_UnknownField(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, listFixtureTOML)
	_, stderr, err := runCmd("list", path, "bogus")
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if !strings.Contains(stderr.String(), "unknown field") {
		t.Errorf("stderr missing hint: %q", stderr.String())
	}
}

func TestCmdVolumes_Listing(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, listFixtureTOML)
	stdout, _, err := runCmd("volumes", path)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	out := stdout.String()
	// Sorted by name: cache then home.
	if !strings.HasPrefix(out, "cache\t") {
		t.Errorf("expected cache first (sorted): %q", out)
	}
	if !strings.Contains(out, "home\t/home/${USERNAME}") {
		t.Errorf("expected home entry: %q", out)
	}
}

func TestCmdGet_FieldAndDefault(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, listFixtureTOML)
	for field, want := range map[string]string{
		"service-name": "dev",
		"username":     "dev",
		"os":           "ubuntu",
		"os-version":   "24.04",
	} {
		stdout, _, err := runCmd("get", path, field)
		if err != nil {
			t.Fatalf("field=%q err = %v", field, err)
		}
		if !strings.Contains(stdout.String(), want+"\n") {
			t.Errorf("field=%q got %q want %q", field, stdout.String(), want)
		}
	}
}

func TestCmdGet_UnknownField(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, listFixtureTOML)
	_, _, err := runCmd("get", path, "bogus")
	if !errors.Is(err, configcli.ErrUnknownField) {
		t.Fatalf("err = %v, want ErrUnknownField", err)
	}
}

const pluginFixtureTOML = `
[metadata]
name = "Demo Plugin"
description = "A demo plugin (https://example.com)"
default = true

[install]
requires_root = true
user_dirs = [".cache", ".config/demo"]
volumes = ["/home/${USERNAME}/.demo", "/etc/demo"]

[version]
version_capable = true

[apt]
packages = ["jq", "curl"]
`

func writeTempPluginDir(t *testing.T, body string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCmdPluginGet_Fields(t *testing.T) {
	t.Parallel()
	dir := writeTempPluginDir(t, pluginFixtureTOML)
	for _, tc := range []struct {
		field, want string
	}{
		{"id", "demo"},
		{"name", "Demo Plugin"},
		{"description", "A demo plugin (https://example.com)"},
		{"default", "true"},
		{"requires-root", "true"},
		{"version-capable", "true"},
	} {
		stdout, _, err := runCmd("plugin-get", dir, tc.field)
		if err != nil {
			t.Fatalf("field=%q err = %v", tc.field, err)
		}
		if !strings.Contains(stdout.String(), tc.want+"\n") {
			t.Errorf("field=%q got %q want %q", tc.field, stdout.String(), tc.want)
		}
	}
}

func TestCmdPluginGet_AcceptsTomlPath(t *testing.T) {
	t.Parallel()
	dir := writeTempPluginDir(t, pluginFixtureTOML)
	stdout, _, err := runCmd("plugin-get", filepath.Join(dir, "plugin.toml"), "id")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "demo\n") {
		t.Errorf("got %q", stdout.String())
	}
}

func TestCmdPluginGet_UnknownField(t *testing.T) {
	t.Parallel()
	dir := writeTempPluginDir(t, pluginFixtureTOML)
	_, _, err := runCmd("plugin-get", dir, "bogus")
	if !errors.Is(err, configcli.ErrUnknownPluginField) {
		t.Fatalf("err = %v, want ErrUnknownPluginField", err)
	}
}

func TestCmdPluginGet_MissingTOML(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("plugin-get", filepath.Join(t.TempDir(), "missing"), "id")
	if !errors.Is(err, configcli.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
}

func TestCmdPluginList_UserDirs(t *testing.T) {
	t.Parallel()
	dir := writeTempPluginDir(t, pluginFixtureTOML)
	stdout, _, err := runCmd("plugin-list", dir, "user-dirs")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, ".cache\n") || !strings.Contains(out, ".config/demo\n") {
		t.Errorf("expected user-dirs lines: %q", out)
	}
}

func TestCmdPluginList_AptPackages(t *testing.T) {
	t.Parallel()
	dir := writeTempPluginDir(t, pluginFixtureTOML)
	stdout, _, err := runCmd("plugin-list", dir, "apt-packages")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "jq\n") {
		t.Errorf("expected jq: %q", stdout.String())
	}
}

func TestCmdPluginList_UnknownField(t *testing.T) {
	t.Parallel()
	dir := writeTempPluginDir(t, pluginFixtureTOML)
	_, _, err := runCmd("plugin-list", dir, "bogus")
	if !errors.Is(err, configcli.ErrUnknownPluginField) {
		t.Fatalf("err = %v, want ErrUnknownPluginField", err)
	}
}

func TestCmdPluginVolumes(t *testing.T) {
	t.Parallel()
	dir := writeTempPluginDir(t, pluginFixtureTOML)
	stdout, _, err := runCmd("plugin-volumes", dir)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "demo\t/home/${USERNAME}/.demo") {
		t.Errorf("expected demo volume entry: %q", out)
	}
	if !strings.Contains(out, "demo\t/etc/demo") {
		t.Errorf("expected /etc/demo entry: %q", out)
	}
}

func TestCmdPluginsTable_OK(t *testing.T) {
	t.Parallel()
	dir := writeTempPluginDir(t, pluginFixtureTOML)
	pluginsRoot := filepath.Dir(dir)
	stdout, _, err := runCmd("plugins-table", pluginsRoot)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "demo\tDemo Plugin\ttrue\t") {
		t.Errorf("expected TSV row: %q", stdout.String())
	}
}

func TestCmdPluginsTable_NotDir(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("plugins-table", "/nonexistent/dir")
	if !errors.Is(err, configcli.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
}

func TestCmdPluginsTable_MissingArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("plugins-table")
	if !errors.Is(err, configcli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestCmdPluginsTable_SkipsBrokenTOML(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// One valid plugin, one with broken TOML — broken one is skipped with
	// a WARNING; the run must still succeed.
	good := filepath.Join(root, "good")
	bad := filepath.Join(root, "bad")
	for _, d := range []string{good, bad} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(good, "plugin.toml"), []byte(pluginFixtureTOML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "plugin.toml"), []byte("this is not toml]]"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runCmd("plugins-table", root)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "good\tDemo Plugin\t") {
		t.Errorf("good plugin row missing: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "WARNING") {
		t.Errorf("expected warning for broken TOML: %q", stderr.String())
	}
}

func TestRoot_FlagErrorWrapsAsUsage(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("--no-such-flag")
	if !errors.Is(err, configcli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestSub_FlagErrorWrapsAsUsage(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("get", "--bogus")
	if !errors.Is(err, configcli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestCmdHasSection(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, listFixtureTOML)
	for section, want := range map[string]string{
		"container":    "true",
		"plugins":      "true",
		"nonexistent":  "false",
		"devcontainer": "false",
	} {
		stdout, _, err := runCmd("has-section", path, section)
		if err != nil {
			t.Fatalf("section=%q err = %v", section, err)
		}
		if !strings.Contains(stdout.String(), want+"\n") {
			t.Errorf("section=%q got %q want %q", section, stdout.String(), want)
		}
	}
}

func TestCmdHasSection_MissingFile(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("has-section", filepath.Join(t.TempDir(), "no.toml"), "container")
	if !errors.Is(err, configcli.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
}

func TestCmdHasSection_MissingArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("has-section")
	if !errors.Is(err, configcli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

const sidecarTOML = `
[container]
service_name = "dev"
username = "dev"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = []

[services.cache]
image = "redis:7"

[services.db]
image = "postgres:16"
`

func TestCmdListSidecars(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, sidecarTOML)
	stdout, _, err := runCmd("list-sidecars", path)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	out := stdout.String()
	// Sorted: cache then db.
	if !strings.HasPrefix(out, "cache\ndb\n") {
		t.Errorf("expected sorted sidecar names, got %q", out)
	}
}

func TestCmdListSidecars_Empty(t *testing.T) {
	t.Parallel()
	path := writeTempTOML(t, noDcTOML)
	stdout, _, err := runCmd("list-sidecars", path)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got %q", stdout.String())
	}
}

func TestCmdListSidecars_MissingArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("list-sidecars")
	if !errors.Is(err, configcli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestCmdPluginVolumes_WarnOnRelative(t *testing.T) {
	t.Parallel()
	body := `
[metadata]
name = "x"
description = "x (http)"
default = false

[install]
requires_root = false
volumes = ["relative/path"]

[version]
version_capable = false
`
	dir := writeTempPluginDir(t, body)
	_, stderr, err := runCmd("plugin-volumes", dir)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stderr.String(), "non-absolute") {
		t.Errorf("expected warning on relative path: %q", stderr.String())
	}
}

func TestCmdFormatRepositories_FromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "repos.json")
	body := `[{"url":"https://github.com/foo/bar","path":"bar"},{"url":"https://github.com/baz/qux","branch":"main","depth":1}]`
	if err := os.WriteFile(jsonPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runCmd("format-repositories", jsonPath)
	if err != nil {
		t.Fatalf("err = %v stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "[repositories]") {
		t.Errorf("missing header: %q", out)
	}
	if !strings.Contains(out, "https://github.com/foo/bar") {
		t.Errorf("missing url: %q", out)
	}
	if !strings.Contains(out, "branch = 'main'") {
		t.Errorf("missing branch: %q", out)
	}
}

func TestCmdFormatRepositories_EmptyInput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(jsonPath, []byte("   \n\t\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runCmd("format-repositories", jsonPath)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty: %q", stdout.String())
	}
}

func TestCmdFormatRepositories_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(jsonPath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := runCmd("format-repositories", jsonPath)
	if !errors.Is(err, configcli.ErrFailure) {
		t.Fatalf("err = %v, want configcli.ErrFailure", err)
	}
}

func TestCmdFormatRepositories_MissingFile(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("format-repositories", "/no/such/file.json")
	if !errors.Is(err, configcli.ErrFailure) {
		t.Fatalf("err = %v, want configcli.ErrFailure", err)
	}
}
