package configcli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sukekyo26/cocoon/internal/logx"
)

// ErrUnknownPluginField is returned when a plugin-get/list field is unknown.
var ErrUnknownPluginField = errors.New("unknown plugin field")

// resolvePluginPath accepts either a directory containing plugin.toml or the
// plugin.toml path itself and returns the file path.
func resolvePluginPath(target string) (string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", target, err)
	}
	if info.IsDir() {
		return filepath.Join(target, "plugin.toml"), nil
	}
	return target, nil
}

// cmdPluginGet emits a single plugin.toml scalar to stdout.
//
// Usage: wsd config plugin-get <plugin.toml-or-dir> <field>
//
//	field ∈ id, name, description, default, requires-root, version-capable
func cmdPluginGet(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 2, "plugin-get", stderr); err != nil {
		return err
	}
	pluginFile, err := resolvePluginPath(args[0])
	if err != nil {
		log.Errorf("ERROR: %s", err)
		return ErrFailure
	}
	data, err := decodeRaw(pluginFile)
	if err != nil {
		log.Errorf("ERROR: %s", err)
		return ErrFailure
	}
	abs, err := filepath.Abs(pluginFile)
	if err != nil {
		log.Errorf("ERROR: resolve %s: %s", pluginFile, err)
		return ErrFailure
	}
	pluginID := filepath.Base(filepath.Dir(abs))
	metadata := asMap(data["metadata"])
	install := asMap(data["install"])
	versionSection := asMap(data["version"])

	var value string
	switch args[1] {
	case "id":
		value = pluginID
	case "name":
		value = asString(metadata["name"], pluginID)
	case "description":
		value = asString(metadata["description"], "")
	case "default":
		value = boolString(asBool(metadata["default"], false))
	case "requires-root":
		value = boolString(asBool(install["requires_root"], false))
	case "version-capable":
		value = boolString(asBool(versionSection["version_capable"], false))
	default:
		log.Errorf(
			"ERROR: unknown field %q (want: id|name|description|default|requires-root|version-capable)",
			args[1])
		return fmt.Errorf("%w: %s", ErrUnknownPluginField, args[1])
	}
	log.Info(value)
	return nil
}

// cmdPluginList emits a plugin.toml array, one element per line.
//
// Usage: wsd config plugin-list <plugin.toml-or-dir> <field>
//
//	field ∈ user-dirs, apt-packages
func cmdPluginList(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 2, "plugin-list", stderr); err != nil {
		return err
	}
	pluginFile, err := resolvePluginPath(args[0])
	if err != nil {
		log.Errorf("ERROR: %s", err)
		return ErrFailure
	}
	data, err := decodeRaw(pluginFile)
	if err != nil {
		log.Errorf("ERROR: %s", err)
		return ErrFailure
	}
	var items []any
	switch args[1] {
	case "user-dirs":
		items = asSliceAny(asMap(data["install"])["user_dirs"])
	case "apt-packages":
		items = asSliceAny(asMap(data["apt"])["packages"])
	default:
		log.Errorf("ERROR: unknown field %q (want: user-dirs|apt-packages)", args[1])
		return fmt.Errorf("%w: %s", ErrUnknownPluginField, args[1])
	}
	printLines(log, items)
	return nil
}

// cmdPluginVolumes emits plugin install.volumes entries as `name<TAB>path`.
// The name is the basename of the path (leading dots stripped), matching the
// derivation done by cmdPlugin.
func cmdPluginVolumes(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 1, "plugin-volumes", stderr); err != nil {
		return err
	}
	pluginFile, err := resolvePluginPath(args[0])
	if err != nil {
		log.Errorf("ERROR: %s", err)
		return ErrFailure
	}
	data, err := decodeRaw(pluginFile)
	if err != nil {
		log.Errorf("ERROR: %s", err)
		return ErrFailure
	}
	abs, err := filepath.Abs(pluginFile)
	if err != nil {
		log.Errorf("ERROR: resolve %s: %s", pluginFile, err)
		return ErrFailure
	}
	pluginID := filepath.Base(filepath.Dir(abs))

	for _, vol := range asSliceAny(asMap(data["install"])["volumes"]) {
		s, ok := vol.(string)
		if !ok {
			continue
		}
		if !strings.HasPrefix(s, "/") {
			log.Errorf("WARNING: Plugin %q has non-absolute volume path: %s", pluginID, s)
		}
		basename := strings.TrimLeft(filepath.Base(strings.TrimRight(s, "/")), ".")
		log.Infof("%s\t%s", basename, s)
	}
	return nil
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
