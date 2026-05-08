package shellrc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/shellrc"
)

func ptr(s string) *string { return &s }

func TestGenerateEmpty(t *testing.T) {
	t.Parallel()
	ctx := &generate.WorkspaceContext{WS: &config.Workspace{}}
	rel, got, err := shellrc.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if rel != "config/.bashrc_custom.generated" {
		t.Errorf("default rel: got %q", rel)
	}
	if got != shellrc.HeaderFor("bash") {
		t.Errorf("empty shell: got %q want header", got)
	}
}

func TestGenerateNilContext(t *testing.T) {
	t.Parallel()
	rel, got, err := shellrc.Generate(nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if rel != "config/.bashrc_custom.generated" {
		t.Errorf("nil-ctx rel: got %q", rel)
	}
	if got != shellrc.HeaderFor("bash") {
		t.Errorf("nil ctx: got %q", got)
	}
}

func TestRelPathFor(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"bash":    "config/.bashrc_custom.generated",
		"zsh":     "config/.zshrc_custom.generated",
		"fish":    "config/config.fish_custom.generated",
		"unknown": "config/.bashrc_custom.generated",
	}
	for shell, want := range cases {
		if got := shellrc.RelPathFor(shell); got != want {
			t.Errorf("RelPathFor(%q) = %q, want %q", shell, got, want)
		}
	}
}

func TestGenerateGoldenBash(t *testing.T) {
	t.Parallel()
	ctx := bashCtx()
	rel, got, err := shellrc.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if rel != "config/.bashrc_custom.generated" {
		t.Errorf("rel: got %q", rel)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "bashrc_full.expected"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}

func TestGenerateGoldenZsh(t *testing.T) {
	t.Parallel()
	ctx := bashCtx()
	ctx.WS.Container.Shell.Default = ptr("zsh")
	rel, got, err := shellrc.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if rel != "config/.zshrc_custom.generated" {
		t.Errorf("rel: got %q", rel)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "zshrc_full.expected"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}

func TestGenerateGoldenFish(t *testing.T) {
	t.Parallel()
	ctx := bashCtx()
	ctx.WS.Container.Shell.Default = ptr("fish")
	rel, got, err := shellrc.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if rel != "config/config.fish_custom.generated" {
		t.Errorf("rel: got %q", rel)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "config_fish_full.expected"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
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
	_, got, err := shellrc.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	wants := []string{
		"export DOLLAR='$HOME'",
		"export EMPTY=''",
		`export WITHQUOTE='it'"'"'s'`,
		"export SPACES='less -R'",
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
	_, got, err := shellrc.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// fish: $-expansion does not happen inside single quotes so $HOME is
	// emitted verbatim; a quote becomes \' and a backslash becomes \\.
	wants := []string{
		"set -gx DOLLAR '$HOME'",
		"set -gx EMPTY ''",
		`set -gx WITHQUOTE 'it\'s'`,
		"set -gx SPACES 'less -R'",
		"set -gx SAFE 'vim'",
		`set -gx BACKSLASH 'a\\b'`,
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
