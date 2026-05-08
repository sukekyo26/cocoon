package verifyartifactscli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/shellrc"
	"github.com/sukekyo26/cocoon/internal/logx"
)

const verifyArtifactsLong = `wsd verify-artifacts — verify generated artifacts match workspace.toml

Loads <workspace.toml> and the four generated artifacts in <output_dir>
(Dockerfile, docker-compose.yml, .devcontainer/devcontainer.json, and the
per-shell rc fragment under config/) and asserts every option declared in
the workspace is reflected in the corresponding artifact.`

// NewCommand returns the cobra command for ` + "`wsd verify-artifacts`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "verify-artifacts <workspace.toml> <output_dir>",
		Short:         "Verify generated artifacts match workspace.toml",
		Long:          verifyArtifactsLong,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) != 2 {
				logx.New(stdout, stderr).Error("Usage: wsd verify-artifacts <workspace.toml> <output_dir>")
				return ErrUsage
			}
			return runVerify(args[0], args[1], stdout, stderr)
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	clihelpers.AttachHelpAlias(cmd)
	return cmd
}

func runVerify(wsPath, outDir string, stdout, stderr io.Writer) error {
	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		return fmt.Errorf("%w: load workspace: %w", ErrFailure, err)
	}

	df, err := os.ReadFile(filepath.Join(outDir, "Dockerfile"))
	if err != nil {
		return fmt.Errorf("%w: read Dockerfile: %w", ErrFailure, err)
	}
	composeBytes, err := os.ReadFile(filepath.Join(outDir, "docker-compose.yml"))
	if err != nil {
		return fmt.Errorf("%w: read docker-compose.yml: %w", ErrFailure, err)
	}
	devcBytes, err := os.ReadFile(filepath.Join(outDir, ".devcontainer", "devcontainer.json"))
	if err != nil {
		return fmt.Errorf("%w: read devcontainer.json: %w", ErrFailure, err)
	}
	//nolint:exhaustruct // verifier only needs WS to derive shell info.
	rcCtx := &generate.WorkspaceContext{WS: ws}
	rcRel := shellrc.RelPathFor(rcCtx.LoginShell())
	rcPath := filepath.Join(outDir, rcRel)
	var bashrc string
	if b, rerr := os.ReadFile(rcPath); rerr == nil {
		bashrc = string(b)
	}
	rcSyntax := rcCtx.RCSyntax()

	var compose map[string]any
	if err := yaml.Unmarshal(composeBytes, &compose); err != nil {
		return fmt.Errorf("%w: parse docker-compose.yml: %w", ErrFailure, err)
	}
	var devc map[string]any
	if err := json.Unmarshal(stripJSONC(devcBytes), &devc); err != nil {
		return fmt.Errorf("%w: parse devcontainer.json: %w", ErrFailure, err)
	}

	//nolint:exhaustruct // failures slice defaults to nil and is appended in-place.
	v := &verifier{ws: ws, df: string(df), compose: compose, devc: devc, bashrc: bashrc, rcSyntax: rcSyntax}
	v.run()

	log := logx.New(stdout, stderr)
	if len(v.failures) > 0 {
		log.Errorf("❌ %d verification failure(s):", len(v.failures))
		for _, msg := range v.failures {
			log.Errorf("  - %s", msg)
		}
		return ErrFailure
	}
	log.Infof("✓ All generated-artifact assertions passed for %s", filepath.Base(wsPath))
	return nil
}
