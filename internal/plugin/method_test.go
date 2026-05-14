package plugin_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

// TestResolveMethod_LegacyPluginReturnsEmpty pins backward compatibility:
// a plugin without [install.methods] resolves to "" (legacy install.sh
// path), even when the workspace passes a method for it.
func TestResolveMethod_LegacyPluginReturnsEmpty(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{}
	got, err := plugin.ResolveMethod(p, "x", map[string]string{"x": "binary"})
	require.NoError(t, err)
	require.Empty(t, got)
}

// TestResolveMethod_WorkspaceOverrideWins pins that an explicit
// workspace override takes precedence over default_method.
func TestResolveMethod_WorkspaceOverrideWins(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Install: plugin.Install{
			DefaultMethod: "official",
			Methods: map[string]plugin.InstallMethod{
				"official": {Description: "x"},
				"binary":   {Description: "y"},
			},
		},
	}
	got, err := plugin.ResolveMethod(p, "x", map[string]string{"x": "binary"})
	require.NoError(t, err)
	require.Equal(t, "binary", got)
}

// TestResolveMethod_DefaultsWhenNoOverride pins that an absent / nil /
// empty-string workspace entry falls back to default_method.
func TestResolveMethod_DefaultsWhenNoOverride(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Install: plugin.Install{
			DefaultMethod: "official",
			Methods: map[string]plugin.InstallMethod{
				"official": {Description: "x"},
			},
		},
	}
	cases := []struct {
		name    string
		methods map[string]string
	}{
		{"nil_map", nil},
		{"empty_map", map[string]string{}},
		{"empty_string_value", map[string]string{"x": ""}},
		{"unrelated_id_set", map[string]string{"other": "binary"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := plugin.ResolveMethod(p, "x", tc.methods)
			require.NoError(t, err)
			require.Equal(t, "official", got)
		})
	}
}

// TestResolveMethod_UnknownMethodIsSentinel pins the error class: the
// caller can distinguish "workspace points at a method the plugin
// doesn't declare" from other failures via errors.Is.
func TestResolveMethod_UnknownMethodIsSentinel(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Install: plugin.Install{
			DefaultMethod: "official",
			Methods: map[string]plugin.InstallMethod{
				"official": {Description: "x"},
			},
		},
	}
	_, err := plugin.ResolveMethod(p, "x", map[string]string{"x": "unknown"})
	require.ErrorIs(t, err, plugin.ErrUnknownMethod)
	require.Contains(t, err.Error(), "unknown")
	require.Contains(t, err.Error(), "x")
}
