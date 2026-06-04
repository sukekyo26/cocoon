// Package plugincli implements the `cocoon plugin` subcommand tree.
//
// Subcommands:
//
//	list       list every plugin available in the layered view
//	show       print the resolved manifest for one plugin id
//	pin        emit (or write in-place) a [plugins].enable entry for one plugin
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
