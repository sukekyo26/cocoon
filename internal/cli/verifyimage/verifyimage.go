// Package verifyimagecli implements `wsd verify-image`.
//
// It boots short-lived containers from a built image (`docker run --rm`) and
// asserts that the image actually contains everything the workspace.toml
// fixture declares: tool binaries for every enabled plugin, pinned tool
// versions, apt packages, locale, git identity, and Dockerfile-hook marker
// files.
//
// Replaces the legacy tests/integration/verify_image_contents.sh, which
// relied on a python TOML reader inside a bash script.
package verifyimagecli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/exec/dockerx"
	"github.com/sukekyo26/cocoon/internal/generate/shellx"
	"github.com/sukekyo26/cocoon/internal/logx"
)

// ErrUsage maps to exit code 2.
var ErrUsage = errors.New("usage error")

// ErrFailure maps to exit code 1.
var ErrFailure = errors.New("failure")

// pluginBin maps a plugin id to the binary that should be on PATH inside the
// built image. Plugins that don't ship a binary (nerd-fonts, custom-ps1, …)
// are intentionally absent — they're skipped.
var pluginBin = map[string]string{
	"docker-cli":    "docker",
	"aws-cli":       "aws",
	"aws-sam-cli":   "sam",
	"github-cli":    "gh",
	"claude-code":   "claude",
	"copilot-cli":   "copilot",
	"proto":         "proto",
	"uv":            "uv",
	"zig":           "zig",
	"rust":          "rustc",
	"go":            "go",
	"lazygit":       "lazygit",
	"starship":      "starship",
	"google-chrome": "google-chrome",
}

// versionCmd maps a plugin id to the shell command that prints its version.
// Only version_capable plugins appear here; the others have no `--version`.
var versionCmd = map[string]string{
	"go":          "go version",
	"lazygit":     "lazygit --version",
	"starship":    "starship --version",
	"zig":         "zig version",
	"uv":          "uv --version",
	"proto":       "proto --version",
	"copilot-cli": "copilot --version",
}

type runner struct {
	image    string
	ws       *config.Workspace
	docker   *dockerx.Client
	stdout   io.Writer
	stderr   io.Writer
	log      *logx.Logger
	failures int
}

func (r *runner) pass(format string, args ...any) {
	r.log.Infof("  ✓ "+format, args...)
}

func (r *runner) fail(format string, args ...any) {
	r.log.Errorf("  ❌ "+format, args...)
	r.failures++
}

// inImage runs the given shell snippet inside a `docker run --rm` of the
// image and returns stdout (trimmed).
//
// IMPORTANT: callers must shell-quote any user-derived value embedded in
// shell via [shellx.ShellQuote]; this function does not parse the script.
func (r *runner) inImage(shell string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := r.docker.RunBash(ctx, r.image, shell)
	if err != nil {
		return out, err //nolint:wrapcheck // dockerx already wraps with the docker prefix.
	}
	return out, nil
}

func (r *runner) verifyTools() {
	r.log.Info("→ Tool presence")
	for _, plugin := range r.ws.Plugins.Enable {
		bin, ok := pluginBin[plugin]
		if !ok {
			continue
		}
		_, err := r.inImage(fmt.Sprintf("command -v %s >/dev/null 2>&1", shellx.ShellQuote(bin)))
		if err == nil {
			r.pass("%s → %s found", plugin, bin)
		} else {
			r.fail("%s → %s NOT found in image", plugin, bin)
		}
	}
}

func (r *runner) verifyPinnedVersions() {
	r.log.Info("→ Pinned versions")
	for plugin, spec := range r.ws.Plugins.Versions {
		if spec.Pin == "" {
			continue
		}
		cmd, ok := versionCmd[plugin]
		if !ok {
			continue
		}
		out, err := r.inImage(cmd)
		if err != nil {
			r.fail("%s: failed to query version: %v", plugin, err)
			continue
		}
		if strings.Contains(out, spec.Pin) {
			r.pass("%s pinned to %s", plugin, spec.Pin)
		} else {
			r.fail("%s: expected version %s, got: %s", plugin, spec.Pin, out)
		}
	}
}

func (r *runner) verifyAptPackages() {
	if r.ws.Apt == nil || len(r.ws.Apt.Packages) == 0 {
		return
	}
	r.log.Info("→ apt packages")
	for _, pkg := range r.ws.Apt.Packages {
		_, err := r.inImage(fmt.Sprintf("dpkg -s %s >/dev/null 2>&1", shellx.ShellQuote(pkg)))
		if err == nil {
			r.pass("apt package installed: %s", pkg)
		} else {
			r.fail("apt package NOT installed: %s", pkg)
		}
	}
}

func (r *runner) verifyLocale() {
	if r.ws.Locale == nil || r.ws.Locale.Lang == nil || *r.ws.Locale.Lang == "" {
		return
	}
	r.log.Info("→ Locale")
	lang := *r.ws.Locale.Lang
	expectedLocale := strings.ToLower(strings.ReplaceAll(lang, "-", ""))
	probe := fmt.Sprintf("locale -a | tr -d '-' | tr '[:upper:]' '[:lower:]' | grep -qx %s",
		shellx.ShellQuote(expectedLocale))
	if _, err := r.inImage(probe); err == nil {
		r.pass("locale generated: %s", lang)
	} else {
		r.fail("locale NOT generated: %s", lang)
	}
	actual, _ := r.inImage(`echo $LANG`) //nolint:errcheck // empty actual fails below.
	if actual == lang {
		r.pass("ENV LANG=%s", lang)
	} else {
		r.fail("ENV LANG: expected %s, got %q", lang, actual)
	}
}

func (r *runner) verifyGitIdentity() {
	if r.ws.Git == nil {
		return
	}
	r.log.Info("→ Git identity")
	if r.ws.Git.UserName != nil && *r.ws.Git.UserName != "" {
		want := *r.ws.Git.UserName
		got, _ := r.inImage("git config --system user.name") //nolint:errcheck // empty got fails below.
		if got == want {
			r.pass("git user.name = %s", want)
		} else {
			r.fail("git user.name: expected %q, got %q", want, got)
		}
	}
	if r.ws.Git.UserEmail != nil && *r.ws.Git.UserEmail != "" {
		want := *r.ws.Git.UserEmail
		got, _ := r.inImage("git config --system user.email") //nolint:errcheck // empty got fails below.
		if got == want {
			r.pass("git user.email = %s", want)
		} else {
			r.fail("git user.email: expected %q, got %q", want, got)
		}
	}
}

func (r *runner) verifyDockerfileHooks() {
	if r.ws.Dockerfile == nil {
		return
	}
	r.log.Info("→ Dockerfile hook markers")
	if r.ws.Dockerfile.PreUserSetup != nil && strings.TrimSpace(*r.ws.Dockerfile.PreUserSetup) != "" {
		if _, err := r.inImage("test -f /etc/marker-pre-user-setup"); err == nil {
			r.pass("[dockerfile].pre_user_setup ran (marker present)")
		} else {
			r.fail("[dockerfile].pre_user_setup did NOT run (marker missing)")
		}
	}
	if r.ws.Dockerfile.PostPlugins != nil && strings.TrimSpace(*r.ws.Dockerfile.PostPlugins) != "" {
		if _, err := r.inImage("test -f /etc/marker-post-plugins"); err == nil {
			r.pass("[dockerfile].post_plugins ran (marker present)")
		} else {
			r.fail("[dockerfile].post_plugins did NOT run (marker missing)")
		}
	}
}
