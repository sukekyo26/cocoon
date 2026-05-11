package generate_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
)

func ptr[T any](v T) *T { return &v }

func TestWorkspaceContext_NilSafe(t *testing.T) {
	t.Parallel()
	c := &generate.WorkspaceContext{}
	if got := c.EnabledPlugins(); got == nil || len(got) != 0 {
		t.Errorf("EnabledPlugins should be empty non-nil slice, got %v", got)
	}
	if c.ServiceName() != "dev" {
		t.Errorf("ServiceName = %q, want \"dev\"", c.ServiceName())
	}
	if c.Username() != "developer" {
		t.Errorf("Username = %q, want \"developer\"", c.Username())
	}
	if got := c.ComposeForwardPorts(); got != nil {
		t.Errorf("ComposeForwardPorts should be nil, got %v", got)
	}
	if got := c.DevcontainerForwardPorts(); len(got) != 1 || got[0] != 3000 {
		t.Errorf("DevcontainerForwardPorts = %v, want [3000]", got)
	}
	if c.Resources() != nil {
		t.Error("Resources should be nil")
	}
	if c.LocaleLang() != "" {
		t.Error("LocaleLang should be empty")
	}
	if c.LocaleTimezone() != "" {
		t.Error("LocaleTimezone should be empty")
	}
	if got := c.UserEnv(); len(got) != 0 {
		t.Errorf("UserEnv = %v, want empty", got)
	}
	if got := c.Mounts(); got == nil || len(got) != 0 {
		t.Errorf("Mounts should be empty non-nil slice, got %v", got)
	}
	if got := c.CustomVolumes(); len(got) != 0 {
		t.Errorf("CustomVolumes = %v, want empty", got)
	}
	if got := c.AptExtraPackages(); got == nil || len(got) != 0 {
		t.Errorf("AptExtraPackages should be empty non-nil slice, got %v", got)
	}
	if c.GitUserName() != "" {
		t.Error("GitUserName should be empty")
	}
	if c.GitUserEmail() != "" {
		t.Error("GitUserEmail should be empty")
	}
	if c.DockerfilePreUserSetup() != "" {
		t.Error("DockerfilePreUserSetup should be empty")
	}
	if c.DockerfilePostPlugins() != "" {
		t.Error("DockerfilePostPlugins should be empty")
	}
	if got := c.PluginVersionOverrides(); len(got) != 0 {
		t.Errorf("PluginVersionOverrides = %v, want empty", got)
	}
	if got := c.Sidecars(); len(got) != 0 {
		t.Errorf("Sidecars = %v, want empty", got)
	}
	if got := c.SidecarNames(); len(got) != 0 {
		t.Errorf("SidecarNames = %v, want empty", got)
	}
	if got := c.DevcontainerOverrides(); len(got) != 0 {
		t.Errorf("DevcontainerOverrides = %v, want empty", got)
	}
}

func TestWorkspaceContext_PopulatedAccessors(t *testing.T) {
	t.Parallel()
	ws := &config.Workspace{
		Container: config.ContainerSpec{
			ServiceName: "myapp", Username: "alice", Image: "ubuntu", ImageVersion: "24.04",
			Resources: &config.Resources{Memory: ptr("8g")},
		},
		Plugins: config.PluginsSpec{
			Enable: []string{"go", "node"},
			Versions: map[string]config.PluginVersionOverride{
				"go": {Pin: "1.22.0"},
			},
		},
		Ports: &config.PortsSpec{Forward: []any{
			"8080:8080",
			"127.0.0.1:9090:9090/tcp",
			map[string]any{"target": int64(5432), "mode": "host"},
			"3000-3005:3000-3005",
		}},
		Apt:     &config.AptSpec{Packages: []string{"jq", "curl"}},
		Volumes: map[string]string{"cache": "/var/cache"},
		Env:     map[string]string{"FOO": "bar", "BAZ": "qux"},
		Mounts: []config.Mount{
			{Source: "/host", Target: "/cont", Readonly: false},
		},
		Locale: &config.LocaleSpec{
			Lang:     ptr("ja_JP.UTF-8"),
			Timezone: ptr("Asia/Tokyo"),
		},
		Git: &config.GitIdentitySpec{
			UserName:  ptr("Alice"),
			UserEmail: ptr("a@example.com"),
		},
		Dockerfile: &config.DockerfileSpec{
			PreUserSetup: ptr("RUN echo pre"),
			PostPlugins:  ptr("RUN echo post"),
		},
		Services: map[string]config.SidecarService{
			"db":    {Image: "postgres"},
			"cache": {Image: "redis"},
		},
		Devcontainer: config.Devcontainer{"customizations": map[string]any{}},
	}
	c := &generate.WorkspaceContext{WS: ws}

	if got := c.EnabledPlugins(); strings.Join(got, ",") != "go,node" {
		t.Errorf("EnabledPlugins = %v", got)
	}
	if c.ServiceName() != "myapp" {
		t.Errorf("ServiceName = %q", c.ServiceName())
	}
	if c.Username() != "alice" {
		t.Errorf("Username = %q", c.Username())
	}
	if got := c.ComposeForwardPorts(); len(got) != 4 {
		t.Errorf("ComposeForwardPorts len = %d, want 4 (got %v)", len(got), got)
	} else {
		if got[0].Short != "8080:8080" {
			t.Errorf("[0].Short = %q", got[0].Short)
		}
		if !got[2].IsLong() {
			t.Errorf("[2] should be long form")
		}
		if got[2].Long["target"] != 5432 || got[2].Long["mode"] != "host" {
			t.Errorf("[2].Long = %v", got[2].Long)
		}
	}
	// Devcontainer must skip the host-mode entry and the port range, keeping
	// only the two single-port entries (8080, 9090).
	if got := c.DevcontainerForwardPorts(); len(got) != 2 || got[0] != 8080 || got[1] != 9090 {
		t.Errorf("DevcontainerForwardPorts = %v, want [8080 9090]", got)
	}
	if r := c.Resources(); r == nil || r.Memory == nil || *r.Memory != "8g" {
		t.Errorf("Resources = %+v", r)
	}
	if c.LocaleLang() != "ja_JP.UTF-8" {
		t.Errorf("LocaleLang = %q", c.LocaleLang())
	}
	if c.LocaleTimezone() != "Asia/Tokyo" {
		t.Errorf("LocaleTimezone = %q", c.LocaleTimezone())
	}
	if got := c.UserEnv(); got["FOO"] != "bar" || got["BAZ"] != "qux" {
		t.Errorf("UserEnv = %v", got)
	}
	if got := c.Mounts(); len(got) != 1 || got[0].Source != "/host" {
		t.Errorf("Mounts = %v", got)
	}
	if got := c.CustomVolumes(); got["cache"] != "/var/cache" {
		t.Errorf("CustomVolumes = %v", got)
	}
	if got := c.AptExtraPackages(); strings.Join(got, ",") != "jq,curl" {
		t.Errorf("AptExtraPackages = %v", got)
	}
	if c.GitUserName() != "Alice" {
		t.Errorf("GitUserName = %q", c.GitUserName())
	}
	if c.GitUserEmail() != "a@example.com" {
		t.Errorf("GitUserEmail = %q", c.GitUserEmail())
	}
	if c.DockerfilePreUserSetup() != "RUN echo pre" {
		t.Errorf("DockerfilePreUserSetup = %q", c.DockerfilePreUserSetup())
	}
	if c.DockerfilePostPlugins() != "RUN echo post" {
		t.Errorf("DockerfilePostPlugins = %q", c.DockerfilePostPlugins())
	}
	if got := c.PluginVersionOverrides(); got["go"].Pin != "1.22.0" {
		t.Errorf("PluginVersionOverrides = %+v", got)
	}
	if got := c.Sidecars(); len(got) != 2 {
		t.Errorf("Sidecars = %v", got)
	}
	if got := c.SidecarNames(); strings.Join(got, ",") != "cache,db" {
		t.Errorf("SidecarNames = %v, want sorted [cache db]", got)
	}
	got := c.DevcontainerOverrides()
	if _, ok := got["customizations"]; !ok {
		t.Errorf("DevcontainerOverrides missing customizations: %v", got)
	}
}

func TestWorkspaceContext_ResolveLocale(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		lang         *string
		wantGenList  string
		wantLang     string
		wantLanguage string
	}{
		{"unset", nil, "en_US.UTF-8", "en_US.UTF-8", "en_US:en"},
		{"en_US.UTF-8", ptr("en_US.UTF-8"), "en_US.UTF-8", "en_US.UTF-8", "en_US:en"},
		{"ja_JP.UTF-8", ptr("ja_JP.UTF-8"), "en_US.UTF-8 ja_JP.UTF-8", "ja_JP.UTF-8", "ja_JP:en"},
		{"no_dot", ptr("ja_JP"), "en_US.UTF-8 ja_JP", "ja_JP", "ja_JP:en"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := &config.Workspace{}
			if tc.lang != nil {
				ws.Locale = &config.LocaleSpec{Lang: tc.lang}
			}
			c := &generate.WorkspaceContext{WS: ws}
			gen, lang, language := c.ResolveLocale()
			if gen != tc.wantGenList || lang != tc.wantLang || language != tc.wantLanguage {
				t.Errorf("got (%q,%q,%q), want (%q,%q,%q)",
					gen, lang, language, tc.wantGenList, tc.wantLang, tc.wantLanguage)
			}
		})
	}
}

func TestWorkspaceContext_BuildEnvironment(t *testing.T) {
	t.Parallel()
	t.Run("inserts_TZ_when_locale_set", func(t *testing.T) {
		t.Parallel()
		ws := &config.Workspace{
			Locale: &config.LocaleSpec{Timezone: ptr("Asia/Tokyo")},
			Env:    map[string]string{"FOO": "bar"},
		}
		c := &generate.WorkspaceContext{WS: ws}
		got := c.BuildEnvironment()
		if got[0] != "CONTAINER_SERVICE_NAME=${CONTAINER_SERVICE_NAME}" {
			t.Errorf("got[0] = %q", got[0])
		}
		joined := strings.Join(got, ",")
		if !strings.Contains(joined, "TZ=Asia/Tokyo") {
			t.Errorf("missing TZ override: %v", got)
		}
		if !strings.Contains(joined, "FOO=bar") {
			t.Errorf("missing user env: %v", got)
		}
	})
	t.Run("env_TZ_overridden_emits_warning", func(t *testing.T) {
		t.Parallel()
		ws := &config.Workspace{
			Locale: &config.LocaleSpec{Timezone: ptr("Asia/Tokyo")},
			Env:    map[string]string{"TZ": "UTC"},
		}
		var warnings bytes.Buffer
		c := &generate.WorkspaceContext{WS: ws, Warnings: &warnings}
		got := c.BuildEnvironment()
		if !strings.Contains(strings.Join(got, ","), "TZ=Asia/Tokyo") {
			t.Errorf("expected locale TZ to win: %v", got)
		}
		if !strings.Contains(warnings.String(), "WARNING") {
			t.Errorf("expected warning, got %q", warnings.String())
		}
	})
	t.Run("env_TZ_only", func(t *testing.T) {
		t.Parallel()
		ws := &config.Workspace{
			Env: map[string]string{"TZ": "UTC"},
		}
		c := &generate.WorkspaceContext{WS: ws}
		got := c.BuildEnvironment()
		if !strings.Contains(strings.Join(got, ","), "TZ=UTC") {
			t.Errorf("expected env TZ: %v", got)
		}
	})
}

func TestWorkspaceContext_HomeFileMounts(t *testing.T) {
	t.Parallel()
	t.Run("nil_when_unset", func(t *testing.T) {
		t.Parallel()
		c := &generate.WorkspaceContext{WS: &config.Workspace{}}
		if got := c.HomeFileMounts(); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("emits_home_interpolation", func(t *testing.T) {
		t.Parallel()
		ws := &config.Workspace{
			HomeFiles: &config.HomeFilesSpec{
				Files: []string{".claude.json", ".gemini/oauth_creds.json"},
			},
		}
		c := &generate.WorkspaceContext{WS: ws}
		got := c.HomeFileMounts()
		if len(got) != 2 {
			t.Fatalf("expected 2 mounts, got %d", len(got))
		}
		const prefix = "${HOME:?HOME must be set on the host}"
		if got[0].Source != prefix+"/.claude.json" {
			t.Errorf("source[0] = %q, want %q", got[0].Source, prefix+"/.claude.json")
		}
		if got[0].Target != "/home/${USERNAME}/.claude.json" {
			t.Errorf("target[0] = %q", got[0].Target)
		}
		if got[1].Source != prefix+"/.gemini/oauth_creds.json" {
			t.Errorf("source[1] = %q", got[1].Source)
		}
		if got[1].Readonly {
			t.Error("home_files mounts must be RW")
		}
	})
	t.Run("preserves_order", func(t *testing.T) {
		t.Parallel()
		ws := &config.Workspace{
			HomeFiles: &config.HomeFilesSpec{
				Files: []string{".z", ".a", "deep/nested/file.json"},
			},
		}
		c := &generate.WorkspaceContext{WS: ws}
		got := c.HomeFileMounts()
		if len(got) != 3 {
			t.Fatalf("expected 3 mounts, got %d", len(got))
		}
		for i, want := range []string{".z", ".a", "deep/nested/file.json"} {
			if !strings.HasSuffix(got[i].Source, "/"+want) {
				t.Errorf("got[%d].Source = %q, want suffix /%s", i, got[i].Source, want)
			}
			if got[i].Target != "/home/${USERNAME}/"+want {
				t.Errorf("got[%d].Target = %q", i, got[i].Target)
			}
		}
	})
}
