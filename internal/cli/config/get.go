package configcli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sukekyo26/cocoon/internal/logx"
)

// ErrUnknownField is returned when a `get`/`list` field name is not recognised.
var ErrUnknownField = errors.New("unknown field")

// cmdGet emits a single workspace.toml scalar to stdout (one line, no key prefix).
//
// Usage: wsd config get <workspace.toml> <field>
//
//	field ∈ service-name, username, os, os-version
func cmdGet(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 2, "get", stderr); err != nil {
		return err
	}
	data, err := decodeRaw(args[0])
	if err != nil {
		log.Errorf("ERROR: %s", err)
		return ErrFailure
	}
	container := asMap(data["container"])
	var value string
	switch args[1] {
	case "service-name":
		value = asString(container["service_name"], "dev")
	case "username":
		value = asString(container["username"], "developer")
	case "os":
		value = asString(container["os"], "ubuntu")
	case "os-version":
		value = asString(container["os_version"], "24.04")
	default:
		log.Errorf("ERROR: unknown field %q (want: service-name|username|os|os-version)", args[1])
		return fmt.Errorf("%w: %s", ErrUnknownField, args[1])
	}
	log.Info(value)
	return nil
}

// cmdList emits a workspace.toml array, one element per line.
//
// Usage: wsd config list <workspace.toml> <field>
//
//	field ∈ plugins, forward-ports, apt-extra
func cmdList(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 2, "list", stderr); err != nil {
		return err
	}
	data, err := decodeRaw(args[0])
	if err != nil {
		log.Errorf("ERROR: %s", err)
		return ErrFailure
	}
	var items []any
	switch args[1] {
	case "plugins":
		items = asSliceAny(asMap(data["plugins"])["enable"])
	case "forward-ports":
		ports := asMap(data["ports"])
		if forward, ok := ports["forward"]; ok {
			items = asSliceAny(forward)
		} else {
			items = []any{"3000:3000"}
		}
	case "apt-extra":
		items = asSliceAny(asMap(data["apt"])["packages"])
	default:
		log.Errorf("ERROR: unknown field %q (want: plugins|forward-ports|apt-extra)", args[1])
		return fmt.Errorf("%w: %s", ErrUnknownField, args[1])
	}
	printLines(log, items)
	return nil
}

// cmdVolumes emits [volumes] entries as `name<TAB>path`, one per line.
func cmdVolumes(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 1, "volumes", stderr); err != nil {
		return err
	}
	data, err := decodeRaw(args[0])
	if err != nil {
		log.Errorf("ERROR: %s", err)
		return ErrFailure
	}
	volumes := asMap(data["volumes"])
	for _, k := range sortedKeys(volumes) {
		log.Infof("%s\t%s", k, asString(volumes[k], ""))
	}
	return nil
}

func printLines(log *logx.Logger, items []any) {
	for _, it := range items {
		log.Info(scalarString(it))
	}
}

// scalarString formats a TOML scalar (string/int/bool/float) the way
// `wsd config list` should emit it on stdout. Long-form port tables collapse
// to docker-compose short syntax when target+published are present, falling
// back to a `key=value` summary when they aren't.
func scalarString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int64:
		return fmt.Sprintf("%d", x)
	case int:
		return fmt.Sprintf("%d", x)
	case float64:
		return fmt.Sprintf("%v", x)
	case map[string]any:
		return formatPortMap(x)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatPortMap(m map[string]any) string {
	target := portInt(m["target"])
	published := portInt(m["published"])
	hostIP, _ := m["host_ip"].(string) //nolint:errcheck // type assert ok-pattern; default "" is intended.
	proto, _ := m["protocol"].(string) //nolint:errcheck // type assert ok-pattern; default "" is intended.
	if target > 0 && published > 0 {
		s := fmt.Sprintf("%d:%d", published, target)
		if hostIP != "" {
			s = hostIP + ":" + s
		}
		if proto != "" {
			s += "/" + proto
		}
		return s
	}
	parts := []string{}
	for _, k := range []string{"target", "published", "host_ip", "protocol", "mode"} {
		if v, ok := m[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	}
	return strings.Join(parts, " ")
}

func portInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		// Only accept integer-valued floats. 3.9 → 0 so a malformed config
		// falls back to the key=value summary instead of silently collapsing
		// into "3:3" short syntax.
		if n == float64(int(n)) {
			return int(n)
		}
	}
	return 0
}
