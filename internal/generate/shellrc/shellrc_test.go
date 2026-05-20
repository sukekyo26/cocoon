package shellrc_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/shellrc"
)

func ptr(s string) *string { return &s }

func TestRenderEmptyStillEmitsBootstrap(t *testing.T) {
	t.Parallel()
	got, err := shellrc.RenderDockerfileBlock(&generate.WorkspaceContext{WS: &config.Workspace{}})
	if err != nil {
		t.Fatalf("RenderDockerfileBlock: %v", err)
	}
	wants := []string{
		`RUN <<COCOON_RC_BLOCK`,
		`cat >>"$HOME/.bashrc" <<'COCOON_RC'`,
		`[ -f "$HOME/.cocoon/.shellrc" ] && . "$HOME/.cocoon/.shellrc"`,
		`COCOON_RC_BLOCK`,
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
	if strings.Contains(got, "# Environment variables") {
		t.Errorf("must not emit env section when env is empty:\n%s", got)
	}
	if strings.Contains(got, "# Aliases") {
		t.Errorf("must not emit alias section when aliases are empty:\n%s", got)
	}
}

func TestRenderNilContext(t *testing.T) {
	t.Parallel()
	got, err := shellrc.RenderDockerfileBlock(nil)
	if err != nil {
		t.Fatalf("RenderDockerfileBlock: %v", err)
	}
	if got != "" {
		t.Errorf("nil ctx must produce empty block, got %q", got)
	}
}

func TestRenderBash(t *testing.T) {
	t.Parallel()
	got, err := shellrc.RenderDockerfileBlock(bashCtx())
	if err != nil {
		t.Fatalf("RenderDockerfileBlock: %v", err)
	}
	wants := []string{
		"# Inject [container.shell] env/aliases plus persistent shellrc bootstrap",
		`RUN <<COCOON_RC_BLOCK`,
		`cat >>"$HOME/.bashrc" <<'COCOON_RC'`,
		"export EDITOR=vim",
		`export PAGER="less -R"`,
		"alias gs='git status'",
		`[ -f "$HOME/.cocoon/.shellrc" ] && . "$HOME/.cocoon/.shellrc"`,
		"COCOON_RC",
		"COCOON_RC_BLOCK",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
}

func TestRenderZshTargetsZshrc(t *testing.T) {
	t.Parallel()
	ctx := bashCtx()
	ctx.WS.Container.Shell.Default = ptr("zsh")
	got, err := shellrc.RenderDockerfileBlock(ctx)
	if err != nil {
		t.Fatalf("RenderDockerfileBlock: %v", err)
	}
	if !strings.Contains(got, `cat >>"$HOME/.zshrc"`) {
		t.Errorf("zsh target missing in:\n%s", got)
	}
}

func TestRenderFishUsesSetGxAndFishConfig(t *testing.T) {
	t.Parallel()
	ctx := bashCtx()
	ctx.WS.Container.Shell.Default = ptr("fish")
	got, err := shellrc.RenderDockerfileBlock(ctx)
	if err != nil {
		t.Fatalf("RenderDockerfileBlock: %v", err)
	}
	wants := []string{
		`cat >>"$HOME/.config/fish/config.fish"`,
		`set -gx EDITOR "vim"`,
		`set -gx PAGER "less -R"`,
		"alias gs 'git status'",
		`test -f "$HOME/.cocoon/.shellrc.fish"; and source "$HOME/.cocoon/.shellrc.fish"`,
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
	if strings.Contains(got, "export ") {
		t.Errorf("fish output must not contain POSIX export:\n%s", got)
	}
}

func TestPosixQuotingPolicy(t *testing.T) {
	t.Parallel()
	ctx := &generate.WorkspaceContext{
		WS: &config.Workspace{
			Container: config.ContainerSpec{
				Shell: &config.ContainerShellSpec{
					Env: map[string]string{
						"EMPTY":     "",
						"SAFE":      "vim",
						"SPACES":    "less -R",
						"WITHQUOTE": "it's",
						"DOLLAR":    "$HOME",
					},
				},
			},
		},
	}
	got, err := shellrc.RenderDockerfileBlock(ctx)
	if err != nil {
		t.Fatalf("RenderDockerfileBlock: %v", err)
	}
	// Env values are emitted with double-quote semantics so $HOME expands
	// at shell start-up; an apostrophe inside double quotes is harmless.
	wants := []string{
		`export DOLLAR="$HOME"`,
		`export EMPTY=""`,
		`export WITHQUOTE="it's"`,
		`export SPACES="less -R"`,
		"export SAFE=vim",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
}

func TestFishQuotingPolicy(t *testing.T) {
	t.Parallel()
	ctx := &generate.WorkspaceContext{
		WS: &config.Workspace{
			Container: config.ContainerSpec{
				Shell: &config.ContainerShellSpec{
					Default: ptr("fish"),
					Env: map[string]string{
						"EMPTY":     "",
						"SAFE":      "vim",
						"SPACES":    "less -R",
						"WITHQUOTE": "it's",
						"DOLLAR":    "$HOME",
						"BACKSLASH": `a\b`,
					},
				},
			},
		},
	}
	got, err := shellrc.RenderDockerfileBlock(ctx)
	if err != nil {
		t.Fatalf("RenderDockerfileBlock: %v", err)
	}
	// fish double quotes expand $VAR (parity with bash/zsh) and escape only
	// \ and ". Single quotes inside double quotes are literal.
	wants := []string{
		`set -gx DOLLAR "$HOME"`,
		`set -gx EMPTY ""`,
		`set -gx WITHQUOTE "it's"`,
		`set -gx SPACES "less -R"`,
		`set -gx SAFE "vim"`,
		`set -gx BACKSLASH "a\\b"`,
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
}

func bashCtx() *generate.WorkspaceContext {
	return &generate.WorkspaceContext{
		WS: &config.Workspace{
			Container: config.ContainerSpec{
				Shell: &config.ContainerShellSpec{
					Aliases: map[string]string{
						"gs":  "git status",
						"ga":  "git add",
						"gc":  "git commit",
						"gco": "git checkout",
						"gd":  "git diff",
						"gb":  "git branch",
						"gp":  "git push",
						"gl":  "git log --oneline --graph --decorate",
						"ll":  "ls -lah",
					},
					Env: map[string]string{
						"EDITOR": "vim",
						"PAGER":  "less -R",
					},
				},
			},
		},
	}
}
