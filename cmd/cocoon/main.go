// Command cocoon is the project-aware container workspace generator binary.
//
// It reads workspace.toml from the current project and generates Dockerfile,
// docker-compose.yml and devcontainer.json tailored to that project.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/sukekyo26/cocoon/internal/clean"
	"github.com/sukekyo26/cocoon/internal/cli"
	certificatescli "github.com/sukekyo26/cocoon/internal/cli/certificates"
	cleancli "github.com/sukekyo26/cocoon/internal/cli/clean"
	configcli "github.com/sukekyo26/cocoon/internal/cli/config"
	devcontainercli "github.com/sukekyo26/cocoon/internal/cli/devcontainer"
	doctorcli "github.com/sukekyo26/cocoon/internal/cli/doctor"
	generatecli "github.com/sukekyo26/cocoon/internal/cli/generate"
	plugincli "github.com/sukekyo26/cocoon/internal/cli/plugin"
	rebuildcli "github.com/sukekyo26/cocoon/internal/cli/rebuild"
	repositoriescli "github.com/sukekyo26/cocoon/internal/cli/repositories"
	schemacli "github.com/sukekyo26/cocoon/internal/cli/schema"
	setupcli "github.com/sukekyo26/cocoon/internal/cli/setup"
	tuicli "github.com/sukekyo26/cocoon/internal/cli/tui"
	verifyartifactscli "github.com/sukekyo26/cocoon/internal/cli/verifyartifacts"
	verifyimagecli "github.com/sukekyo26/cocoon/internal/cli/verifyimage"
	workspacecli "github.com/sukekyo26/cocoon/internal/cli/workspace"
	"github.com/sukekyo26/cocoon/internal/rebuild"
	"github.com/sukekyo26/cocoon/internal/setup"
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
	switch {
	case errors.Is(err, tuicli.ErrCanceled), errors.Is(err, clean.ErrCanceled),
		errors.Is(err, rebuild.ErrCanceled), errors.Is(err, workspacecli.ErrCanceled),
		errors.Is(err, plugincli.ErrCanceled), errors.Is(err, setup.ErrCanceled):
		return 130
	case errors.Is(err, devcontainercli.ErrMissingDocker):
		return 2
	case errors.Is(err, devcontainercli.ErrMissingDcCLI):
		return 3
	case errors.Is(err, devcontainercli.ErrMissingDcJSON):
		return 4
	case errors.Is(err, devcontainercli.ErrMissingEnvFile):
		return 5
	case errors.Is(err, configcli.ErrUsage), errors.Is(err, generatecli.ErrUsage), errors.Is(err, tuicli.ErrUsage),
		errors.Is(err, doctorcli.ErrUsage), errors.Is(err, repositoriescli.ErrUsage),
		errors.Is(err, certificatescli.ErrUsage), errors.Is(err, devcontainercli.ErrUsage),
		errors.Is(err, cleancli.ErrUsage), errors.Is(err, rebuildcli.ErrUsage),
		errors.Is(err, workspacecli.ErrUsage), errors.Is(err, plugincli.ErrUsage),
		errors.Is(err, verifyartifactscli.ErrUsage), errors.Is(err, verifyimagecli.ErrUsage),
		errors.Is(err, schemacli.ErrUsage), errors.Is(err, setupcli.ErrUsage),
		errors.Is(err, configcli.ErrUnknownField), errors.Is(err, configcli.ErrUnknownPluginField):
		return 2
	case errors.Is(err, configcli.ErrFailure), errors.Is(err, generatecli.ErrFailure), errors.Is(err, tuicli.ErrFailure),
		errors.Is(err, doctorcli.ErrFailure), errors.Is(err, repositoriescli.ErrFailure),
		errors.Is(err, certificatescli.ErrFailure), errors.Is(err, devcontainercli.ErrFailure),
		errors.Is(err, cleancli.ErrInsideContainer), errors.Is(err, clean.ErrPartial),
		errors.Is(err, clean.ErrConfig), errors.Is(err, clean.ErrPrereq),
		errors.Is(err, rebuildcli.ErrInsideContainer), errors.Is(err, rebuild.ErrConfig),
		errors.Is(err, rebuild.ErrPrereq), errors.Is(err, rebuild.ErrFailure),
		errors.Is(err, setup.ErrInsideContainer),
		errors.Is(err, workspacecli.ErrFailure), errors.Is(err, plugincli.ErrFailure),
		errors.Is(err, verifyartifactscli.ErrFailure), errors.Is(err, verifyimagecli.ErrFailure),
		errors.Is(err, schemacli.ErrFailure):
		return 1
	default:
		fmt.Fprintf(stderr, "cocoon: %v\n", err)
		return 1
	}
}
