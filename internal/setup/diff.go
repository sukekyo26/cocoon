package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sukekyo26/cocoon/internal/config"
	wsdexec "github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/shellrc"
)

func diffRelPaths(wsPath string) []string {
	rcRel := "config/.bashrc_custom.generated"
	if ws, err := config.LoadWorkspace(wsPath); err == nil {
		ctx := &generate.WorkspaceContext{WS: ws}
		rcRel = shellrc.RelPathFor(ctx.LoginShell())
	}
	return []string{
		"docker-compose.yml",
		"Dockerfile",
		".devcontainer/devcontainer.json",
		".devcontainer/docker-compose.yml",
		rcRel,
	}
}

func runDiff(opts Options, wsPath, pluginsDir string) error {
	log := opts.Logger
	tmpDir, err := os.MkdirTemp("", "wsd-setup-")
	if err != nil {
		return fmt.Errorf("mkdir temp: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := opts.Generator.GenerateAll(wsPath, pluginsDir, tmpDir, opts.Stderr); err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	diffFound := false
	for _, rel := range diffRelPaths(wsPath) {
		existing := filepath.Join(opts.WorkspaceDir, rel)
		candidate := filepath.Join(tmpDir, rel)
		if !fileExists(existing) {
			log.Infof("### NEW FILE: %s", rel)
			if data, err := os.ReadFile(candidate); err == nil { //nolint:gosec // candidate is under our own tmpDir.
				log.Print(string(data))
			}
			diffFound = true
			continue
		}
		existData, existErr := os.ReadFile(existing) //nolint:gosec // existing is under opts.WorkspaceDir.
		candData, candErr := os.ReadFile(candidate)  //nolint:gosec // candidate is under our own tmpDir.
		if existErr == nil && candErr == nil && string(existData) == string(candData) {
			continue
		}
		log.Infof("### DIFF: %s", rel)
		// External `diff` exits 1 when files differ — that's the expected
		// outcome here, so the run error is intentionally ignored.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = wsdexec.New().RunWithIO(ctx, wsdexec.RunOptions{ //nolint:errcheck // see comment above.
			Name:   "diff",
			Args:   []string{"-u", existing, candidate},
			Stdin:  nil,
			Stdout: log.Stdout(),
			Stderr: nil,
			Env:    nil,
			Dir:    "",
		})
		cancel()
		log.Info("")
		diffFound = true
	}

	if diffFound {
		log.Info("\nChanges detected. Run ./setup-docker.sh to apply.")
		return ErrDiffFound
	}
	log.Info("No changes.")
	return nil
}
