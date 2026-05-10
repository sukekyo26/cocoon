// Package plugincli implements the `cocoon plugin` subcommand tree.
//
// Subcommands:
//
//	list       list every plugin available in the layered view
//	show       print the resolved manifest for one plugin id
//	pin        emit (or write in-place) a [plugins.versions.<id>] block
//	scaffold   create a new <id>/ directory under .cocoon/plugins from a template
//
// To use an embedded plugin, add its id to [plugins].enable in
// workspace.toml. To customise it, the supported workflow is
// `cocoon plugin scaffold <new-id>` and adapting the logic. With a
// cocoon source checkout, copying internal/plugin/catalog/<id>/
// into the user / project overlay is a shortcut; single-binary
// installs do not include the embedded source on disk.
//
// Each handler writes its output to the supplied stdout/stderr writers and
// returns sentinel errors that the binary boundary maps to exit codes.
package plugincli

import "errors"

// ErrUsage signals a usage error (missing argument, unknown subcommand) and
// maps to exit code 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure (validation failure, write error) and
// maps to exit code 1.
var ErrFailure = errors.New("failure")

// ErrCanceled is returned when the user aborts an interactive prompt
// (Ctrl-C / Esc); maps to exit code 130 at the binary boundary.
var ErrCanceled = errors.New("canceled")
