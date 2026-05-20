// Package shellrc renders the Dockerfile RUN block that injects the
// [container.shell] env/aliases from workspace.toml plus a bootstrap line
// that sources the user's persistent shellrc (~/.cocoon/.shellrc, or
// ~/.cocoon/.shellrc.fish for fish) from the cocoon named volume.
//
// The env/aliases are appended directly to ~/.{shell}rc inside the image so no
// host-side companion artifact is written. Bash and zsh share POSIX-style
// `export K=V` / `alias k='v'` syntax; fish uses `set -gx K V` / `alias k 'v'`.
// Env values are double-quoted via shellx.{Posix,Fish}ExportValue so $HOME /
// $PATH / $(cmd) expand when the shell sources the rc file. Alias bodies use
// single quotes (shellx.{Shell,Fish}Quote) because they are re-parsed by the
// shell at invocation time, so embedded $-references resolve there.
//
// The persistent shellrc bootstrap is appended after the env/aliases so that
// users can override anything cocoon sets by editing ~/.cocoon/.shellrc.
package shellrc

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/shellx"
)

// RenderDockerfileBlock emits the bootstrap unconditionally; env/aliases
// only when configured. Uses a Dockerfile heredoc wrapping a shell
// heredoc to avoid the multi-layer single-quote escaping that nested
// `echo '...' >> file` would require for values like `it's`.
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

// persistentShellrcBootstrap guards the source with an existence check so
// the rc stays valid if the user removes the file. Placed last so user
// edits override cocoon's env/aliases.
func persistentShellrcBootstrap(syntax string) string {
	if syntax == "fish" {
		return `test -f "$HOME/.cocoon/.shellrc.fish"; and source "$HOME/.cocoon/.shellrc.fish"`
	}
	return `[ -f "$HOME/.cocoon/.shellrc" ] && . "$HOME/.cocoon/.shellrc"`
}

func renderEnvLines(env map[string]string, syntax string) []string {
	keys := slices.Sorted(maps.Keys(env))
	out := make([]string, len(keys))
	for i, k := range keys {
		if syntax == "fish" {
			out[i] = "set -gx " + k + " " + shellx.FishExportValue(env[k])
		} else {
			out[i] = "export " + k + "=" + shellx.PosixExportValue(env[k])
		}
	}
	return out
}

func renderAliasLines(aliases map[string]string, syntax string) []string {
	keys := slices.Sorted(maps.Keys(aliases))
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
