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
	"github.com/sukekyo26/cocoon/internal/i18n"
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
	// boundary's job to surface the message. This is also the single point
	// where the active language is known, so localize here. Print first,
	// then map to the numeric exit code expected by callers.
	cat := i18n.New(i18n.Detect())
	logx.New(io.Discard, stderr).Errorf("cocoon: %s", renderError(cat, err))
	switch {
	case errors.Is(err, clihelpers.ErrCanceled):
		return 130
	case errors.Is(err, clihelpers.ErrUsage):
		return 2
	default:
		return 1
	}
}

// renderError localizes any error in the chain that implements i18n.Localizer
// (cocoon-authored LocError / ValidationError); stdlib / 3rd-party errors fall
// back to their English Error() text.
func renderError(cat *i18n.Catalog, err error) string {
	var loc i18n.Localizer
	if errors.As(err, &loc) {
		return loc.Localize(cat)
	}
	return err.Error()
}
