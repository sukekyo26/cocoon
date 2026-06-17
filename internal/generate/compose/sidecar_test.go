package compose_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/compose"
	"github.com/sukekyo26/cocoon/internal/plugin"
	"github.com/sukekyo26/cocoon/internal/warn"
)

// TestGenerate_WithSidecars exercises buildSidecar / buildSidecarVolumes which
// the snapshot fixture does not cover.
func TestGenerate_WithSidecars(t *testing.T) {
	t.Parallel()

	restart := config.RestartUnlessStopped
	seccomp := "unconfined"
	ws := &config.Workspace{
		Container: config.ContainerSpec{
			ServiceName: "dev", Username: "dev", Image: "ubuntu", ImageVersion: "24.04",
		},
		Plugins: config.PluginsSpec{Enable: []string{}},
		Services: map[string]config.SidecarService{
			"db": {
				Image:   "postgres:16",
				Ports:   []any{"5432:5432"},
				Env:     map[string]string{"POSTGRES_DB": "app"},
				Volumes: map[string]string{"pgdata": "/var/lib/postgresql/data"},
				Healthcheck: config.HealthcheckSpec{
					"test":     []any{"CMD-SHELL", "pg_isready"},
					"interval": "5s",
				},
				Restart: &restart,
			},
			"cache": {
				Image:     "redis:7-alpine",
				Ports:     []any{6379},
				DependsOn: []string{"db"},
			},
			"emu": {
				Image:        "redroid/redroid:13.0.0-latest",
				Privileged:   true,
				Devices:      []string{"/dev/binder:/dev/binder"},
				Capabilities: &config.CapabilitiesSpec{Add: []string{"SYS_ADMIN"}, Drop: []string{"NET_RAW"}},
				SecurityOpt:  &config.SecurityOptSpec{Seccomp: &seccomp},
			},
		},
	}

	ctx := &generate.WorkspaceContext{
		WS: ws, PluginsFS: nil, Plugins: map[string]*plugin.Plugin{}, Warnings: warn.New(),
	}
	got, err := compose.Generate(ctx, compose.Options{
		Plugins: map[string]*plugin.Plugin{}, Warnings: warn.New(),
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, sub := range []string{
		"postgres:16",
		"redis:7-alpine",
		"depends_on:",
		"healthcheck:",
		`restart: "unless-stopped"`,
		"pgdata:", // named volume from sidecar volumes
		"redroid/redroid:13.0.0-latest",
		"privileged: true",
		"/dev/binder:/dev/binder",
		"cap_add:",
		"SYS_ADMIN", // cap_add value, not just the key
		"cap_drop:",
		"NET_RAW", // cap_drop value, not just the key
		"security_opt:",
		"seccomp=unconfined",
	} {
		if !strings.Contains(got, sub) {
			t.Errorf("missing %q in:\n%s", sub, got)
		}
	}
}

// TestGenerate_WithResources exercises applyResources which is partially
// covered by the snapshot fixture but not for all field combinations.
func TestGenerate_WithResources(t *testing.T) {
	t.Parallel()
	ptr := func(s string) *string { return &s }
	cpus := 2.5
	pids := 1024
	nofileSoft, nofileHard := 4096, 8192
	ws := &config.Workspace{
		Container: config.ContainerSpec{
			ServiceName: "dev", Username: "dev", Image: "ubuntu", ImageVersion: "24.04",
			Resources: &config.Resources{
				ShmSize:         ptr("2g"),
				PidsLimit:       &pids,
				StopGracePeriod: ptr("30s"),
				CPUs:            &cpus,
				Memory:          ptr("8g"),
				NofileSoft:      &nofileSoft,
				NofileHard:      &nofileHard,
			},
		},
		Plugins: config.PluginsSpec{Enable: []string{}},
	}
	ctx := &generate.WorkspaceContext{WS: ws, Plugins: map[string]*plugin.Plugin{}, Warnings: warn.New()}
	got, err := compose.Generate(ctx, compose.Options{Plugins: map[string]*plugin.Plugin{}, Warnings: warn.New()})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, sub := range []string{"shm_size", "pids_limit", "stop_grace_period", "ulimits"} {
		if !strings.Contains(got, sub) {
			t.Errorf("missing %q in:\n%s", sub, got)
		}
	}
}
