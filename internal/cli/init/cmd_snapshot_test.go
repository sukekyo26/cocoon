//nolint:testpackage // shares pinEnglish helper with cmd_test.go
package initcli

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
)

//nolint:gochecknoglobals // test-only flag scoped to this test file.
var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/init/*.workspace.toml from current init output")

// TestRunInit_Snapshot pins `cocoon init --yes` output across a matrix of
// flag combinations. Re-run with `-update-golden` after intentional writer
// or i18n changes and review the diff before committing.
//
//nolint:paralleltest // each subtest uses t.Chdir, which mutates process cwd
func TestRunInit_Snapshot(t *testing.T) {
	cases := []struct {
		name   string
		golden string
		args   []string
	}{
		{
			name:   "default",
			golden: "default.workspace.toml",
			args: []string{
				"--yes", "--service-name", "dev", "--username", "dev",
				"--image", "ubuntu", "--image-version", "22.04",
				"--mount-root", ".", "--no-devcontainer",
				"--apt-categories", "text-editors,vcs,utilities,compression,build",
			},
		},
		{
			// Same shape as `default` but with --certificates so the live
			// [certificates] enable=true section is emitted instead of the
			// commented template. Pins the opt-in branch end-to-end.
			name:   "default-with-certificates",
			golden: "default-with-certificates.workspace.toml",
			args: []string{
				"--yes", "--service-name", "dev", "--username", "dev",
				"--image", "ubuntu", "--image-version", "22.04",
				"--mount-root", ".", "--no-devcontainer", "--certificates",
				"--apt-categories", "text-editors,vcs,utilities,compression,build",
			},
		},
		{
			name:   "plugins-amd64-full",
			golden: "plugins-amd64-full.workspace.toml",
			args: []string{
				"--yes", "--service-name", "dev", "--username", "dev",
				"--image", "ubuntu", "--image-version", "22.04",
				"--mount-root", ".", "--no-devcontainer",
				"--apt-categories", "text-editors,vcs,utilities,compression,build",
				"--plugins",
				"docker-cli,docker-buildx,aws-cli,aws-sam-cli,github-cli,claude-code,copilot-cli," +
					"proto,mise,uv,bun,node,deno,zig,rust,go,lazygit,starship," +
					"nerd-fonts,google-chrome,terraform,opentofu," +
					"kubectl,helm,shellcheck,shfmt",
				"--plugin-versions",
				"bun=1.3.3,copilot-cli=0.0.369,deno=2.7.14,docker-buildx=0.24.0,go=1.23.4," +
					"helm=3.16.0,kubectl=1.31.0,lazygit=0.44.1,mise=2025.12.0,node=24.15.0," +
					"opentofu=1.9.0,proto=0.46.1,shellcheck=0.10.0,shfmt=3.10.0," +
					"starship=1.21.1,terraform=1.10.5,uv=0.5.7,zig=0.13.0",
			},
		},
		{
			name:   "plugins-arm64-full",
			golden: "plugins-arm64-full.workspace.toml",
			args: []string{
				"--yes", "--service-name", "dev", "--username", "dev",
				"--image", "ubuntu", "--image-version", "22.04",
				"--mount-root", ".", "--no-devcontainer",
				"--apt-categories", "text-editors,vcs,utilities,compression,build",
				"--plugins",
				"docker-cli,docker-buildx,github-cli,claude-code,copilot-cli,proto,mise,uv," +
					"bun,node,deno,rust,go,nerd-fonts,terraform,opentofu," +
					"kubectl,helm,shellcheck,shfmt",
				"--plugin-versions",
				"bun=1.3.3,copilot-cli=0.0.369,deno=2.7.14,docker-buildx=0.24.0,go=1.23.4," +
					"helm=3.16.0,kubectl=1.31.0,mise=2025.12.0,node=24.15.0,opentofu=1.9.0," +
					"proto=0.46.1,shellcheck=0.10.0,shfmt=3.10.0,terraform=1.10.5,uv=0.5.7",
			},
		},
		{
			name:   "plugins-versions-minimal",
			golden: "plugins-versions-minimal.workspace.toml",
			args: []string{
				"--yes", "--service-name", "dev", "--username", "dev",
				"--image", "ubuntu", "--image-version", "22.04",
				"--mount-root", ".", "--no-devcontainer",
				"--apt-categories", "text-editors,vcs,utilities,compression,build",
				"--plugins", "go,uv",
				"--plugin-versions", "go=1.23.4,uv=0.5.7",
			},
		},
		{
			// Pins the active [ports] block path: --ports promotes the
			// commented-out template to a live `forward = [...]` array.
			// Exercises every accepted short-form variant so a regex /
			// validator drift will surface here as a deliberate diff.
			name:   "ports-set",
			golden: "ports-set.workspace.toml",
			args: []string{
				"--yes", "--service-name", "dev", "--username", "dev",
				"--image", "ubuntu", "--image-version", "22.04",
				"--mount-root", ".", "--no-devcontainer",
				"--apt-categories", "text-editors,vcs,utilities,compression,build",
				"--ports",
				"3000,3000-3005,8000:8000,9090-9091:8080-8081,49100:22," +
					"127.0.0.1:8001:8001,127.0.0.1:5000-5010:5000-5010,6060:6060/udp",
			},
		},
	}

	// Resolve goldenDir to an absolute path BEFORE t.Chdir below, otherwise
	// "testdata/..." would resolve against the t.TempDir() and the golden
	// would be written into (and read from) the disposable workspace.
	pkgWD, wdErr := os.Getwd()
	if wdErr != nil {
		t.Fatalf("getwd: %v", wdErr)
	}
	goldenDir := filepath.Join(pkgWD, "testdata", "init")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pinEnglish(t)
			work := t.TempDir()
			t.Chdir(work)

			cmd := NewCommand(io.Discard, io.Discard)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("init %s: %v", tc.name, err)
			}
			got, err := os.ReadFile(filepath.Join(work, "workspace.toml"))
			if err != nil {
				t.Fatalf("read workspace.toml: %v", err)
			}

			goldenPath := filepath.Join(goldenDir, tc.golden)
			if *updateGolden {
				if mkErr := os.MkdirAll(goldenDir, 0o755); mkErr != nil {
					t.Fatalf("mkdir golden: %v", mkErr)
				}
				// gosec G304/G703: goldenPath is built from a hardcoded
				// tc.golden filename; the only attacker-controlled path
				// would be the package wd, which is the test runner's
				// own checkout — not a meaningful threat surface.
				if werr := os.WriteFile(goldenPath, got, 0o600); werr != nil { //nolint:gosec
					t.Fatalf("write golden: %v", werr)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v (run with -update-golden to create)", goldenPath, err)
			}
			if string(got) != string(want) {
				t.Errorf("workspace.toml mismatch for %s\n--- got ---\n%s\n--- want ---\n%s",
					tc.name, got, want)
			}
		})
	}
}
