// Package devcontainer implements the non-interactive prerequisite checks
// and the WSL-aware `devcontainer up` wrapper that lib/devcontainer.sh used
// to provide. Interactive auto-install of the devcontainer CLI remains in
// the bash entry script because it mutates $HOME and offers a y/N prompt
// that needs to bridge directly to the user's terminal.
package devcontainer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wsdexec "github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/exec/dockerx"
)

const (
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

// PrereqResult enumerates which prerequisite is missing so the bash wrapper
// can decide whether to offer an interactive auto-install (only for the
// devcontainer CLI itself).
type PrereqResult int

// PrereqResult values.
const (
	PrereqOK PrereqResult = iota
	PrereqMissingDocker
	PrereqMissingDevcontainerCLI
	PrereqMissingDevcontainerJSON
	PrereqMissingEnv
)

// CheckPrerequisites verifies Docker is reachable, the devcontainer CLI is
// installed, and the workspace contains .devcontainer/devcontainer.json plus
// .env. It writes per-check ✓/✗ lines to w. Returns PrereqOK on success.
func CheckPrerequisites(runner wsdexec.Runner, workspaceDir string, w io.Writer) PrereqResult {
	if r := checkDocker(runner, w); r != PrereqOK {
		return r
	}
	if r := checkDevcontainerCLI(w); r != PrereqOK {
		return r
	}
	if !fileExists(filepath.Join(workspaceDir, ".devcontainer", "devcontainer.json")) {
		fmt.Fprintf(w, "  %s✗%s devcontainer.json not found\n", colorRed, colorReset)
		return PrereqMissingDevcontainerJSON
	}
	fmt.Fprintf(w, "  %s✓%s devcontainer.json\n", colorGreen, colorReset)

	if !fileExists(filepath.Join(workspaceDir, ".env")) {
		fmt.Fprintf(w, "  %s✗%s .env not found\n", colorRed, colorReset)
		return PrereqMissingEnv
	}
	fmt.Fprintf(w, "  %s✓%s .env\n", colorGreen, colorReset)

	return PrereqOK
}

func checkDocker(runner wsdexec.Runner, w io.Writer) PrereqResult {
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Fprintf(w, "  %s✗%s Docker is not installed\n", colorRed, colorReset)
		return PrereqMissingDocker
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := dockerx.New(runner).Info(ctx); err != nil {
		fmt.Fprintf(w, "  %s✗%s Docker daemon not reachable\n", colorRed, colorReset)
		return PrereqMissingDocker
	}
	fmt.Fprintf(w, "  %s✓%s Docker\n", colorGreen, colorReset)
	return PrereqOK
}

func checkDevcontainerCLI(w io.Writer) PrereqResult {
	// Replicate _ensure_devcontainer_path: prepend $HOME/.devcontainers/bin if
	// it exists so the host PATH lookup succeeds without re-sourcing the shell.
	if home, err := os.UserHomeDir(); err == nil {
		dcBin := filepath.Join(home, ".devcontainers", "bin")
		if dirExists(dcBin) && !pathContains(dcBin) {
			_ = os.Setenv("PATH", dcBin+string(os.PathListSeparator)+os.Getenv("PATH"))
			fmt.Fprintf(w, "  %s!%s Added %s to PATH for this run\n", colorYellow, colorReset, dcBin)
		}
	}
	if _, err := exec.LookPath("devcontainer"); err != nil {
		fmt.Fprintf(w, "  %s✗%s devcontainer CLI not found\n", colorRed, colorReset)
		return PrereqMissingDevcontainerCLI
	}
	fmt.Fprintf(w, "  %s✓%s devcontainer CLI\n", colorGreen, colorReset)
	return PrereqOK
}

// IsWSL detects WSL1/WSL2 environments via the same heuristics as the bash
// is_wsl helper (kernel string + WSL_DISTRO_NAME).
func IsWSL() bool {
	if name := os.Getenv("WSL_DISTRO_NAME"); name != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	s := strings.ToLower(string(data))
	return strings.Contains(s, "microsoft") || strings.Contains(s, "wsl")
}

// ErrDockerMissing is returned by Up when WSL is detected but `docker` is
// not on PATH (the workaround would have nothing to point --docker-path at).
var ErrDockerMissing = errors.New("docker binary not found on PATH (required for WSL workaround)")

// Up runs `devcontainer <args...>` with the WSL workaround applied when
// applicable. It exec's stdin/stdout/stderr through to the caller so the
// CLI's TTY interaction is preserved. A 10-minute timeout is applied because
// the initial image build can take a long time on a fresh host.
func Up(runner wsdexec.Runner, args []string, stdout, stderr io.Writer) error {
	final := args
	env := os.Environ()
	// The WSL workaround is Linux-specific (it pins DOCKER_HOST to a unix
	// socket and tells the devcontainer CLI to use the host's docker
	// binary). IsWSL() already returns false on darwin, but the GOOS guard
	// makes the intent explicit and survives future IsWSL() refactors.
	if runtime.GOOS == "linux" && IsWSL() {
		dockerPath, err := exec.LookPath("docker")
		if err != nil {
			return ErrDockerMissing
		}
		env = append(env, "DOCKER_HOST=unix:///var/run/docker.sock")
		final = append(append([]string{}, args...), "--docker-path", dockerPath)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	err := runner.RunWithIO(ctx, wsdexec.RunOptions{
		Name:   "devcontainer",
		Args:   final,
		Stdin:  os.Stdin,
		Stdout: stdout,
		Stderr: stderr,
		Env:    env,
		Dir:    "",
	})
	if err != nil {
		return fmt.Errorf("devcontainer: %w", err)
	}
	return nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func pathContains(dir string) bool {
	for _, p := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if p == dir {
			return true
		}
	}
	return false
}
