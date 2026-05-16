// Command cocoon is the project-aware container workspace generator binary.
//
// It reads workspace.toml from the current project and generates Dockerfile,
// docker-compose.yml and devcontainer.json tailored to that project.
package main

import (
	"errors"
	"io"
	"os"

	"github.com/sukekyo26/cocoon/internal/cli"
	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	app := cli.New(version.Get(), stdout, stderr)
	return exitCode(app.Execute(args), stderr)
}

func exitCode(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	// Subcommands run with cobra's SilenceErrors=true, so it is the binary
	// boundary's job to surface the message. Print first, then map to the
	// numeric exit code expected by callers.
	logx.New(io.Discard, stderr).Errorf("cocoon: %v", err)
	switch {
	case errors.Is(err, clihelpers.ErrCanceled):
		return 130
	case errors.Is(err, clihelpers.ErrUsage):
		return 2
	default:
		return 1
	}
}
