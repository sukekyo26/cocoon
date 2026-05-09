package compose

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate/yamlx"
)

// buildSidecar mirrors ComposeGenerator._build_sidecar_services for one
// service. It returns (sidecar service node, top-level volume entries).
func buildSidecar(name string, spec config.SidecarService) (*yaml.Node, []yamlx.Pair) {
	pairs := []yamlx.Pair{
		{Key: "image", Value: yamlx.QuotedIfSpecial(spec.Image)},
		{Key: "init", Value: yamlx.Bool(true)},
	}
	if len(spec.Ports) > 0 {
		items := make([]*yaml.Node, 0, len(spec.Ports))
		for _, p := range spec.Ports {
			items = append(items, yamlx.QuotedIfSpecial(scalarString(p)))
		}
		pairs = append(pairs, yamlx.Pair{Key: "ports", Value: yamlx.Seq(items...)})
	}
	if len(spec.Env) > 0 {
		keys := make([]string, 0, len(spec.Env))
		for k := range spec.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		items := make([]*yaml.Node, 0, len(keys))
		for _, k := range keys {
			items = append(items, yamlx.QuotedIfSpecial(k+"="+spec.Env[k]))
		}
		pairs = append(pairs, yamlx.Pair{Key: "environment", Value: yamlx.Seq(items...)})
	}

	mounts, vols := buildSidecarVolumes(name, spec)
	if len(mounts) > 0 {
		pairs = append(pairs, yamlx.Pair{Key: "volumes", Value: yamlx.Seq(mounts...)})
	}
	if spec.Command != nil {
		pairs = append(pairs, yamlx.Pair{Key: "command", Value: yamlx.QuotedIfSpecial(scalarString(spec.Command))})
	}
	if len(spec.DependsOn) > 0 {
		items := make([]*yaml.Node, 0, len(spec.DependsOn))
		for _, d := range spec.DependsOn {
			items = append(items, yamlx.QuotedIfSpecial(d))
		}
		pairs = append(pairs, yamlx.Pair{Key: "depends_on", Value: yamlx.Seq(items...)})
	}
	if len(spec.Healthcheck) > 0 {
		pairs = append(pairs, yamlx.Pair{Key: "healthcheck", Value: anyMap(spec.Healthcheck)})
	}
	if spec.Restart != nil {
		pairs = append(pairs, yamlx.Pair{Key: "restart", Value: yamlx.QuotedIfSpecial(string(*spec.Restart))})
	}
	return yamlx.Map(pairs...), vols
}

func buildSidecarVolumes(name string, spec config.SidecarService) ([]*yaml.Node, []yamlx.Pair) {
	keys := make([]string, 0, len(spec.Volumes))
	for k := range spec.Volumes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	mounts := make([]*yaml.Node, 0, len(keys)+len(spec.Mounts))
	vols := make([]yamlx.Pair, 0, len(keys))
	for _, k := range keys {
		path := spec.Volumes[k]
		ns := name + "_" + k
		mounts = append(mounts, yamlx.QuotedIfSpecial(ns+":"+path))
		vols = append(vols, yamlx.Pair{
			Key:   ns,
			Value: namedVolume(fmt.Sprintf("${COMPOSE_PROJECT_NAME}_%s_%s", name, k)),
		})
	}
	for _, m := range spec.Mounts {
		mount := m.Source + ":" + m.Target
		if m.Readonly {
			mount += ":ro"
		}
		mounts = append(mounts, yamlx.QuotedIfSpecial(mount))
	}
	return mounts, vols
}

func scalarString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int64:
		return fmt.Sprintf("%d", x)
	case int:
		return fmt.Sprintf("%d", x)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// anyMap converts a map[string]any into an ordered yaml node by sorting keys.
// Used for healthcheck entries which the schema declares as extra-allow.
func anyMap(m map[string]any) *yaml.Node {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]yamlx.Pair, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, yamlx.Pair{Key: k, Value: anyNode(m[k])})
	}
	return yamlx.Map(pairs...)
}

func anyNode(v any) *yaml.Node {
	switch x := v.(type) {
	case string:
		return yamlx.QuotedIfSpecial(x)
	case bool:
		return yamlx.Bool(x)
	case int:
		return yamlx.Int(x)
	case int64:
		return yamlx.Int(int(x))
	case float64:
		return floatNode(x)
	case []any:
		items := make([]*yaml.Node, 0, len(x))
		for _, item := range x {
			items = append(items, anyNode(item))
		}
		return yamlx.Seq(items...)
	case map[string]any:
		return anyMap(x)
	default:
		return yamlx.QuotedIfSpecial(fmt.Sprintf("%v", v))
	}
}
