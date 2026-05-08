package verifyimagecli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	wsdexec "github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/exec/dockerx"
	"github.com/sukekyo26/cocoon/internal/logx"
)

const verifyImageLong = `wsd verify-image — verify a built image contains everything workspace.toml declares

Boots short-lived containers (` + "`docker run --rm`" + `) from <image-tag> and asserts that
the image actually contains every tool, version pin, apt package, locale,
git identity, and Dockerfile-hook marker that <workspace.toml> declares.`

// NewCommand returns the cobra command for ` + "`wsd verify-image`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	return NewCommandWithRunner(wsdexec.New(), stdout, stderr)
}

// NewCommandWithRunner builds the command with an injected runner so tests can
// substitute a [wsdexec.RecordingRunner].
func NewCommandWithRunner(execRunner wsdexec.Runner, stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "verify-image <image-tag> <workspace.toml> <verify-pins:true|false>",
		Short:         "Verify a built image contains everything workspace.toml declares",
		Long:          verifyImageLong,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) != 3 {
				logx.New(stdout, stderr).Error("Usage: wsd verify-image <image-tag> <workspace.toml> <verify-pins:true|false>")
				return ErrUsage
			}
			return runVerifyImage(execRunner, args[0], args[1], args[2], stdout, stderr)
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

func runVerifyImage(execRunner wsdexec.Runner, image, fixture, verifyPins string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	ws, err := config.LoadWorkspace(fixture)
	if err != nil {
		return fmt.Errorf("%w: load workspace: %w", ErrFailure, err)
	}

	//nolint:exhaustruct // failures slice defaults to nil and is appended in-place.
	r := &runner{image: image, ws: ws, docker: dockerx.New(execRunner), stdout: stdout, stderr: stderr, log: log}
	r.verifyTools()
	if verifyPins == "true" {
		r.verifyPinnedVersions()
	}
	r.verifyAptPackages()
	r.verifyLocale()
	r.verifyGitIdentity()
	r.verifyDockerfileHooks()

	log.Info("")
	if r.failures > 0 {
		log.Errorf("❌ %d image-content assertion(s) failed.", r.failures)
		return ErrFailure
	}
	log.Info("✓ All image-content assertions passed.")
	return nil
}
