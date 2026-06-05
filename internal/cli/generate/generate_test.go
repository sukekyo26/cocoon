package generatecli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	generatecli "github.com/sukekyo26/cocoon/internal/cli/generate"
	"github.com/sukekyo26/cocoon/internal/warn"
)

// runPipeline drives the LoadContext → BuildArtifacts → WriteArtifacts
// sequence the same way `cocoon gen` does, used by every test below in
// place of the long-gone `cocoon generate-all` cobra command.
func runPipeline(t *testing.T, wsPath, pluginsDir, outDir string) error {
	t.Helper()
	ctx, err := generatecli.LoadContext(wsPath, os.DirFS(pluginsDir), pluginsDir, warn.New())
	if err != nil {
		return err
	}
	arts, err := generatecli.BuildArtifacts(ctx)
	if err != nil {
		return err
	}
	return generatecli.WriteArtifacts(arts, outDir)
}

// TestRun_Variants exercises `cocoon gen` end-to-end with a handful of
// workspace.toml shapes and asserts substrings on the generated files.
func TestRun_Variants(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	pluginsDir := filepath.Join(repoRoot, "internal", "plugin", "catalog")

	type expect struct {
		path           string   // relative to outDir
		mustContain    []string // substrings that MUST appear
		mustNotContain []string // substrings that MUST NOT appear
	}

	type tc struct {
		name               string
		workspace          string   // workspace.toml body
		extras             []seed   // extra files to drop into outDir before run
		useEmptyPluginsDir bool     // when true, create a fresh empty plugins/ under tempdir; certs probe uses dir(pluginsDir)
		assert             []expect // post-run substring assertions
	}

	cases := []tc{
		{
			name: "all_artifacts_no_plugins",
			workspace: tomlBase("svc-all", "alice", nil) + `
[apt]
packages = []
`,
			assert: []expect{
				{path: ".devcontainer/docker-compose.yml", mustContain: []string{
					"svc-all", "init: true", `stop_grace_period: "30s"`,
					`shm_size: "1gb"`, "pids_limit: 4096",
				}, mustNotContain: []string{"{{", "${UID}", "${GID}", "group_add"}},
				{path: ".devcontainer/Dockerfile", mustContain: []string{
					"# syntax=docker/dockerfile:1.7",
					"type=cache,target=/var/cache/apt",
					"type=cache,target=/var/lib/apt",
					"useradd -m -s /bin/bash -u 1000 -g 1000",
					"ENV COCOON_USER=${USERNAME}",
					`ENV COCOON_WORKSPACE="/home/alice/workspace/svc-all"`,
					"ENTRYPOINT", "CMD",
				}, mustNotContain: []string{
					"{{", "Install Docker CLI", "Install AWS CLI", "Install Zig",
					"DOCKER_GID", "ARG UID", "ARG GID", "apt-get clean",
				}},
				{path: ".devcontainer/devcontainer.json", mustContain: []string{
					"svc-all", `"remoteUser": "alice"`, `"updateRemoteUserUID": false`,
				}, mustNotContain: []string{"{{"}},
				{path: ".devcontainer/docker-compose.yml", mustContain: []string{"svc-all"}, mustNotContain: []string{"{{"}},
				{path: ".devcontainer/.env", mustContain: []string{
					"CONTAINER_SERVICE_NAME=svc-all", "USERNAME=alice",
					"IMAGE=ubuntu", "IMAGE_VERSION=24.04",
				}, mustNotContain: []string{"UID=", "GID=", "DOCKER_GID="}},
				{path: ".devcontainer/docker-entrypoint.sh", mustContain: []string{
					"#!/bin/bash", "$HOME/.image-local", "setpriv",
				}},
				{path: ".devcontainer/manage.sh", mustContain: []string{
					"#!/usr/bin/env bash", "prune-cache",
					"docker compose -f", "down --volumes --rmi local",
				}},
			},
		},
		{
			name: "partial_plugins",
			workspace: tomlBase("svc-partial", "bob", []string{"docker-cli", "github-cli"}) + `
[apt]
packages = []
`,
			assert: []expect{
				{path: ".devcontainer/Dockerfile", mustContain: []string{
					"Docker CLI", "GitHub CLI",
				}, mustNotContain: []string{
					"{{", "Install AWS CLI", "Install AWS SAM CLI", "Install Zig",
					"ARG DOCKER_GID", "DOCKER_GID",
				}},
				{path: ".devcontainer/docker-compose.yml", mustContain: []string{"svc-partial"}},
				{path: ".devcontainer/devcontainer.json", mustContain: []string{"svc-partial", "bob"}},
			},
		},
		{
			name: "apt_extra_packages_before_locale_gen",
			workspace: tomlBase("svc-apt", "u", nil) + `
[apt]
packages = ["vim-nox", "tmux"]
`,
			assert: []expect{
				{path: ".devcontainer/Dockerfile", mustContain: []string{"vim-nox", "tmux", "locale-gen"}, mustNotContain: []string{"{{APT_EXTRA_PACKAGES}}"}},
			},
		},
		{
			name: "plugin_apt_packages_present_when_enabled",
			workspace: tomlBase("svc-proto", "u", []string{"proto"}) + `
[apt]
packages = []
`,
			assert: []expect{
				{path: ".devcontainer/Dockerfile", mustContain: []string{"build-essential", "libssl-dev"}, mustNotContain: []string{"{{APT_PLUGIN_PACKAGES}}"}},
			},
		},
		{
			name: "plugin_apt_packages_absent_when_disabled",
			workspace: tomlBase("svc-noproto", "u", nil) + `
[apt]
packages = []
`,
			assert: []expect{
				{path: ".devcontainer/Dockerfile", mustNotContain: []string{"build-essential", "libssl-dev", "lsb-release"}},
			},
		},
		{
			name: "locale_lang_override",
			workspace: tomlBase("svc-loc", "u", nil) + `
[apt]
packages = []

[locale]
lang = "ja_JP.UTF-8"
`,
			assert: []expect{
				{path: ".devcontainer/Dockerfile", mustContain: []string{
					`sed -i -E -e 's|^# *(en_US\.UTF-8 UTF-8)$|\1|' -e 's|^# *(ja_JP\.UTF-8 UTF-8)$|\1|' /etc/locale.gen`,
					"&& locale-gen \\",
					"ENV LANG=ja_JP.UTF-8",
					"ENV LANGUAGE=ja_JP:en",
					"ENV LC_ALL=ja_JP.UTF-8",
				}, mustNotContain: []string{
					"locale-gen en_US.UTF-8 ja_JP.UTF-8",
				}},
			},
		},
		{
			name: "compose_extensions",
			workspace: tomlBase("svc-ext", "u", nil) + `
[apt]
packages = []

[container.resources]
shm_size = "2gb"
pids_limit = 8192
stop_grace_period = "60s"
cpus = 4.0
memory = "8gb"
nofile_soft = 131072

[env]
FOO = "bar"

[[mounts]]
source = "~/.gitconfig"
target = "/home/u/.gitconfig"
readonly = true

[[mounts]]
source = "/etc/corp-ca"
target = "/usr/local/share/ca-certificates/corp"

[locale]
timezone = "Asia/Tokyo"
`,
			assert: []expect{
				{path: ".devcontainer/docker-compose.yml", mustContain: []string{
					"FOO=bar", "TZ=Asia/Tokyo",
					`shm_size: "2gb"`, "pids_limit: 8192", `stop_grace_period: "60s"`,
					"cpus: 4.0", `mem_limit: "8gb"`, "soft: 131072",
					".gitconfig:/home/u/.gitconfig:ro",
					"/etc/corp-ca:/usr/local/share/ca-certificates/corp",
				}},
			},
		},
		{
			// User cert install is sourced from ~/.cocoon/certs/ via
			// docker-compose's additional_contexts, opt-in through
			// [certificates] enable = true. The cert install RUN block is
			// emitted only when the workspace opts in; the assertion here
			// covers the enabled branch.
			name: "certificates_section_enabled",
			workspace: tomlBase("svc-cert", "u", nil) + `
[apt]
packages = []

[certificates]
enable = true
`,
			useEmptyPluginsDir: true,
			assert: []expect{
				{path: ".devcontainer/Dockerfile", mustContain: []string{
					"# Install custom CA certificates from ~/.cocoon/certs/",
					"--mount=type=bind,from=cocoon_user_certs",
					"update-ca-certificates",
					"SSL_CERT_FILE",
				}},
				{path: ".devcontainer/docker-compose.yml", mustContain: []string{
					"additional_contexts:",
					"cocoon_user_certs:",
					"${HOME:?HOME must be set",
				}},
				{path: ".devcontainer/devcontainer.json", mustContain: []string{
					`"initializeCommand": "mkdir -p \"${HOME:?HOME must be set`,
				}},
			},
		},
		{
			// Default (no [certificates] section) → no cert wiring at all.
			// Teams that never deal with corp CAs commit cert-free
			// artifacts. Mirrors the explicit-disabled case below.
			name: "certificates_section_default_off",
			workspace: tomlBase("svc-no-cert", "u", nil) + `
[apt]
packages = []
`,
			useEmptyPluginsDir: true,
			assert: []expect{
				{path: ".devcontainer/Dockerfile", mustNotContain: []string{
					"cocoon_user_certs",
					"SSL_CERT_FILE",
					"/usr/local/share/ca-certificates/cocoon-user",
				}},
				{path: ".devcontainer/docker-compose.yml", mustNotContain: []string{
					"additional_contexts",
					"cocoon_user_certs",
				}},
				{path: ".devcontainer/devcontainer.json", mustNotContain: []string{
					"initializeCommand",
				}},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			work := t.TempDir()
			pdir := pluginsDir
			if c.useEmptyPluginsDir {
				pdir = filepath.Join(work, "plugins")
				if err := os.MkdirAll(pdir, 0o755); err != nil {
					t.Fatalf("mkdir plugins: %v", err)
				}
			}
			wsPath := filepath.Join(work, "workspace.toml")
			if err := os.WriteFile(wsPath, []byte(c.workspace), 0o600); err != nil {
				t.Fatalf("write workspace.toml: %v", err)
			}
			for _, s := range c.extras {
				abs := filepath.Join(work, s.Rel)
				if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
					t.Fatalf("mkdir extras: %v", err)
				}
				if err := os.WriteFile(abs, []byte(s.Body), 0o600); err != nil {
					t.Fatalf("write extras: %v", err)
				}
			}

			if err := runPipeline(t, wsPath, pdir, work); err != nil {
				t.Fatalf("Run: %v", err)
			}

			for _, e := range c.assert {
				body, err := os.ReadFile(filepath.Join(work, e.path))
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
		})
	}
}

// TestRun_PluginConflictPropagatesAsFailure exercises the conflict branch in
// loadContext: plugin "a" declares "b" in metadata.conflicts.
func TestRun_PluginConflictPropagatesAsFailure(t *testing.T) {
	t.Parallel()
	work := t.TempDir()

	pluginsDir := filepath.Join(work, "plugins")
	tomlBody := func(id, conflictsLine string) string {
		return `
[metadata]
name = "` + id + `"
description = "demo (https://example.com)"
default = false
` + conflictsLine + `

[install]
requires_root = false

[version]
version_capable = false
`
	}
	for _, p := range []struct {
		id, conflicts string
	}{
		{"a", `conflicts = ["b"]`},
		{"b", ""},
	} {
		dir := filepath.Join(pluginsDir, p.id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(tomlBody(p.id, p.conflicts)), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "install.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil { //nolint:gosec
			t.Fatal(err)
		}
	}

	wsPath := filepath.Join(work, "workspace.toml")
	body := `[container]
service_name = "dev"
username = "dev"
image = "ubuntu"
image_version = "24.04"

[plugins]
enable = ["a", "b"]
`
	if err := os.WriteFile(wsPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := runPipeline(t, wsPath, pluginsDir, work); err == nil {
		t.Fatal("expected conflict error, got nil")
	}
}

func TestRun_MissingPluginsDir(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	wsPath := filepath.Join(work, "workspace.toml")
	body := `[container]
service_name = "dev"
username = "dev"
image = "ubuntu"
image_version = "24.04"

[plugins]
enable = []
`
	if err := os.WriteFile(wsPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	// Pluginsdir does not exist; LoadEnabledFromFS with empty enable list still
	// succeeds, so this exercises the "no enabled plugins" no-op path.
	if err := runPipeline(t, wsPath, filepath.Join(work, "no-such-plugins"), work); err != nil {
		t.Fatalf("expected success with no enabled plugins, got %v", err)
	}
}

// TestRun_BadTOMLFailsBeforeWriting verifies the generator surfaces an
// error and does not produce any output file when workspace.toml is invalid.
// Atomic placement (staging dir + rename) is the wrapper's responsibility,
// covered by tests/integration/test_wrappers.sh.
func TestRun_BadTOMLFailsBeforeWriting(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	pluginsDir := filepath.Join(repoRoot, "internal", "plugin", "catalog")

	work := t.TempDir()
	wsPath := filepath.Join(work, "workspace.toml")
	// Missing required [container] section.
	if err := os.WriteFile(wsPath, []byte("[plugins]\nenable = []\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := runPipeline(t, wsPath, pluginsDir, work); err == nil {
		t.Fatal("expected error, got nil")
	}

	for _, name := range []string{"Dockerfile", "docker-compose.yml"} {
		if _, statErr := os.Stat(filepath.Join(work, name)); statErr == nil {
			t.Errorf("%s should not have been written", name)
		}
	}
}

type seed struct {
	Rel  string
	Body string
}

func tomlBase(serviceName, username string, plugins []string) string {
	var pluginsList strings.Builder
	pluginsList.WriteString("[")
	for i, p := range plugins {
		if i > 0 {
			pluginsList.WriteString(", ")
		}
		pluginsList.WriteString(`"`)
		pluginsList.WriteString(p)
		pluginsList.WriteString(`"`)
	}
	pluginsList.WriteString("]")

	var b strings.Builder
	b.WriteString("[container]\n")
	b.WriteString(`service_name = "` + serviceName + "\"\n")
	b.WriteString(`username = "` + username + "\"\n")
	b.WriteString("image = \"ubuntu\"\n")
	b.WriteString("image_version = \"24.04\"\n\n")
	b.WriteString("[plugins]\n")
	b.WriteString("enable = " + pluginsList.String() + "\n\n")
	b.WriteString("[ports]\n")
	b.WriteString("forward = []\n")
	return b.String()
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}
