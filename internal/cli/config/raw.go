package configcli

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// decodeRawFile reads a TOML file into an untyped map. It mirrors Python's
// tomllib.load behavior used by the loose-shape subcommands (workspace,
// plugin, has-section, etc.).
func decodeRawFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is supplied by the trusted caller.
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	out := make(map[string]any)
	if err := toml.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return out, nil
}

// asMap returns v cast to map[string]any if possible, else an empty map.
func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// asSliceAny returns v as []any. TOML decoders may yield typed slices for
// homogeneous arrays, so we widen them.
func asSliceAny(v any) []any {
	switch s := v.(type) {
	case []any:
		return s
	case []string:
		out := make([]any, len(s))
		for i, e := range s {
			out[i] = e
		}
		return out
	case []int64:
		out := make([]any, len(s))
		for i, e := range s {
			out[i] = e
		}
		return out
	case []bool:
		out := make([]any, len(s))
		for i, e := range s {
			out[i] = e
		}
		return out
	default:
		return nil
	}
}

// asString coerces a TOML scalar to a Go string. Returns the fallback if v
// is missing or of the wrong type.
func asString(v any, fallback string) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fallback
}

// asBool coerces a TOML scalar to a Go bool. Returns fallback otherwise.
//
//nolint:unparam // fallback is reserved for future callers; kept for API symmetry with asString/asInt.
func asBool(v any, fallback bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return fallback
}

// asInt coerces TOML int64 (the canonical decoded type) to int. Returns
// fallback when the value is missing or wrong-typed.
func asInt(v any, fallback int) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	default:
		return fallback
	}
}
