// Package shellrc renders the Dockerfile RUN block that injects the
// [container.shell] env/aliases from workspace.toml plus a bootstrap line
// that sources the user's persistent shellrc (~/.cocoon/.shellrc, or
// ~/.cocoon/.shellrc.fish for fish) from the cocoon named volume.
//
// The cocoon design (local/plan.md) abolishes the old workspace-docker pattern
// of writing a *.generated companion file on the host that the container's rc
// sources at startup; instead the contents are appended to ~/.{shell}rc inside
// the image so no host-side artifact remains. Bash and zsh share POSIX-style
// `export K=V` / `alias k='v'` syntax (anchored on shellx.ShellQuote); fish
// uses `set -gx K V` / `alias k 'v'` (anchored on shellx.FishQuote).
//
// The persistent shellrc bootstrap is appended after the env/aliases so that
// users can override anything cocoon sets by editing ~/.cocoon/.shellrc.
package shellrc

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/shellx"
)

// RenderDockerfileBlock returns the RUN block that appends [container.shell]
// env/aliases plus the persistent ~/.cocoon/.shellrc bootstrap to the
// login-shell rc file inside the image. The bootstrap is always emitted; the
// env/aliases sections are only emitted when configured.
//
// The generated form is a Dockerfile heredoc (`RUN <<MARKER ... MARKER`)
// containing a shell-level heredoc, which avoids the multi-layer single-quote
// escaping that nested `echo '...' >> file` would require for values such as
// `it's`. Dockerfile heredocs are supported by BuildKit since the 1.4 syntax
// (cocoon emits `# syntax=docker/dockerfile:1.7`).
func RenderDockerfileBlock(ctx *generate.WorkspaceContext) (string, error) {
	if ctx == nil {
		return "", nil
	}
	aliases := ctx.ShellAliases()
	env := ctx.ShellEnv()

	var inner strings.Builder
	inner.WriteString("# Auto-generated from [container.shell] of workspace.toml.\n")
	if len(env) > 0 {
		inner.WriteString("\n# Environment variables\n")
		for _, line := range renderEnvLines(env, ctx.RCSyntax()) {
			inner.WriteString(line)
			inner.WriteByte('\n')
		}
	}
	if len(aliases) > 0 {
		inner.WriteString("\n# Aliases\n")
		for _, line := range renderAliasLines(aliases, ctx.RCSyntax()) {
			inner.WriteString(line)
			inner.WriteByte('\n')
		}
	}
	inner.WriteString("\n# Source persistent user shellrc from the cocoon named volume\n")
	inner.WriteString(persistentShellrcBootstrap(ctx.RCSyntax()))
	inner.WriteByte('\n')

	var b strings.Builder
	b.WriteString("# Inject [container.shell] env/aliases plus persistent shellrc bootstrap\n")
	b.WriteString("RUN <<COCOON_RC_BLOCK\n")
	fmt.Fprintf(&b, "cat >>\"$HOME/%s\" <<'COCOON_RC'\n", ctx.RCFilePath())
	b.WriteString(inner.String())
	b.WriteString("COCOON_RC\n")
	b.WriteString("COCOON_RC_BLOCK")
	return b.String(), nil
}

// persistentShellrcBootstrap returns the single line that sources
// ~/.cocoon/.shellrc (POSIX) or ~/.cocoon/.shellrc.fish (fish), guarded by
// an existence check so the rc file stays valid even if the user removes
// the file. Placed last in the rc so user edits override cocoon's own
// env/aliases settings.
func persistentShellrcBootstrap(syntax string) string {
	if syntax == "fish" {
		return `test -f "$HOME/.cocoon/.shellrc.fish"; and source "$HOME/.cocoon/.shellrc.fish"`
	}
	return `[ -f "$HOME/.cocoon/.shellrc" ] && . "$HOME/.cocoon/.shellrc"`
}

func renderEnvLines(env map[string]string, syntax string) []string {
	keys := sortedKeys(env)
	out := make([]string, len(keys))
	for i, k := range keys {
		if syntax == "fish" {
			out[i] = "set -gx " + k + " " + shellx.FishQuote(env[k])
		} else {
			out[i] = "export " + k + "=" + shellx.ShellQuote(env[k])
		}
	}
	return out
}

func renderAliasLines(aliases map[string]string, syntax string) []string {
	keys := sortedKeys(aliases)
	out := make([]string, len(keys))
	for i, k := range keys {
		if syntax == "fish" {
			out[i] = "alias " + k + " " + shellx.FishQuote(aliases[k])
		} else {
			out[i] = "alias " + k + "=" + shellx.ShellQuote(aliases[k])
		}
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
