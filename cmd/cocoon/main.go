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
	cleancli "github.com/sukekyo26/cocoon/internal/cli/clean"
	configcli "github.com/sukekyo26/cocoon/internal/cli/config"
	downcli "github.com/sukekyo26/cocoon/internal/cli/down"
	execcli "github.com/sukekyo26/cocoon/internal/cli/exec"
	generatecli "github.com/sukekyo26/cocoon/internal/cli/generate"
	initcli "github.com/sukekyo26/cocoon/internal/cli/init"
	logscli "github.com/sukekyo26/cocoon/internal/cli/logs"
	plugincli "github.com/sukekyo26/cocoon/internal/cli/plugin"
	rebuildcli "github.com/sukekyo26/cocoon/internal/cli/rebuild"
	selfupdatecli "github.com/sukekyo26/cocoon/internal/cli/selfupdate"
	setupcli "github.com/sukekyo26/cocoon/internal/cli/setup"
	upcli "github.com/sukekyo26/cocoon/internal/cli/up"
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
	case errors.Is(err, clean.ErrCanceled),
		errors.Is(err, rebuild.ErrCanceled),
		errors.Is(err, plugincli.ErrCanceled),
		errors.Is(err, setup.ErrCanceled):
		return 130
	case errors.Is(err, configcli.ErrUsage), errors.Is(err, generatecli.ErrUsage),
		errors.Is(err, cleancli.ErrUsage), errors.Is(err, rebuildcli.ErrUsage),
		errors.Is(err, plugincli.ErrUsage), errors.Is(err, setupcli.ErrUsage),
		errors.Is(err, upcli.ErrUsage), errors.Is(err, downcli.ErrUsage),
		errors.Is(err, logscli.ErrUsage), errors.Is(err, execcli.ErrUsage),
		errors.Is(err, initcli.ErrUsage), errors.Is(err, selfupdatecli.ErrUsage),
		errors.Is(err, configcli.ErrUnknownField), errors.Is(err, configcli.ErrUnknownPluginField):
		return 2
	case errors.Is(err, configcli.ErrFailure), errors.Is(err, generatecli.ErrFailure),
		errors.Is(err, cleancli.ErrInsideContainer), errors.Is(err, clean.ErrPartial),
		errors.Is(err, clean.ErrConfig), errors.Is(err, clean.ErrPrereq),
		errors.Is(err, rebuildcli.ErrInsideContainer), errors.Is(err, rebuild.ErrConfig),
		errors.Is(err, rebuild.ErrPrereq), errors.Is(err, rebuild.ErrFailure),
		errors.Is(err, setup.ErrInsideContainer),
		errors.Is(err, plugincli.ErrFailure),
		errors.Is(err, upcli.ErrFailure), errors.Is(err, downcli.ErrFailure),
		errors.Is(err, logscli.ErrFailure), errors.Is(err, execcli.ErrFailure),
		errors.Is(err, initcli.ErrFailure),
		errors.Is(err, selfupdatecli.ErrFailure):
		return 1
	default:
		fmt.Fprintf(stderr, "cocoon: %v\n", err)
		return 1
	}
}
