//nolint:testpackage // exercises unexported parsePluginMethods / writePluginMethods.
package initcli

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// methodFixturePlugins returns a synthetic plugin map that includes one
// plugin with two declared install methods and one legacy single-method
// plugin. The shape mirrors what PR 5 (copilot-cli dogfooding) will
// produce in the real catalog.
func methodFixturePlugins() map[string]*plugin.Plugin {
	return map[string]*plugin.Plugin{
		"multi": {
			Metadata: plugin.Metadata{Name: "multi", Description: "two methods", URL: "https://example.test"},
			Install: plugin.Install{
				DefaultMethod: "official",
				Methods: map[string]plugin.InstallMethod{
					"official": {Description: "Install via upstream installer"},
					"binary":   {Description: "Direct binary download from releases"},
				},
			},
		},
		"legacy": {
			Metadata: plugin.Metadata{Name: "legacy", Description: "single install.sh", URL: "https://example.test"},
			Install:  plugin.Install{},
		},
	}
}

func TestParsePluginMethods_Empty(t *testing.T) {
	t.Parallel()
	plugins := methodFixturePlugins()
	out, err := parsePluginMethods("", plugins, []string{"multi"})
	if err != nil {
		t.Fatalf("empty input: %v", err)
	}
	if out == nil {
		t.Errorf("empty input should return non-nil empty map (nilnil), got nil")
	}
	if len(out) != 0 {
		t.Errorf("empty input should yield zero entries, got %v", out)
	}
}

func TestParsePluginMethods_HappyPath(t *testing.T) {
	t.Parallel()
	plugins := methodFixturePlugins()
	out, err := parsePluginMethods("multi=binary", plugins, []string{"multi"})
	if err != nil {
		t.Fatalf("happy path: %v", err)
	}
	if got := out["multi"]; got != "binary" {
		t.Errorf("multi=binary should land as %q, got %q", "binary", got)
	}
}

func TestParsePluginMethods_WhitespaceTolerated(t *testing.T) {
	t.Parallel()
	plugins := methodFixturePlugins()
	out, err := parsePluginMethods("  multi = binary  ", plugins, []string{"multi"})
	if err != nil {
		t.Fatalf("whitespace: %v", err)
	}
	if got := out["multi"]; got != "binary" {
		t.Errorf("whitespace stripped wrong: got %q", got)
	}
}

// TestParsePluginMethods_Errors keeps every error path in one table so the
// matrix is easy to extend. Each case must hit ErrUsage and the failure
// message must mention the offending input verbatim (so the user can grep
// it in their flag string).
func TestParsePluginMethods_Errors(t *testing.T) {
	t.Parallel()
	plugins := methodFixturePlugins()

	cases := []struct {
		name    string
		raw     string
		enabled []string
		wantSub string
	}{
		{"missing_equals", "multi", []string{"multi"}, "must be <id>=<method>"},
		{"double_equals", "multi==binary", []string{"multi"}, "must be <id>=<method>"},
		{"empty_method", "multi=", []string{"multi"}, "must be <id>=<method>"},
		{"empty_id", "=binary", []string{"multi"}, "must be <id>=<method>"},
		{"unknown_plugin", "ghost=binary", []string{"multi"}, "unknown plugin"},
		{"not_enabled", "multi=binary", []string{"legacy"}, "must also appear in --plugins"},
		{"no_methods_declared", "legacy=binary", []string{"legacy"}, "has no [install.methods]"},
		{"unknown_method", "multi=nonexistent", []string{"multi"}, "no method \"nonexistent\""},
		{"duplicate_id", "multi=binary,multi=official", []string{"multi"}, "duplicate id"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parsePluginMethods(tc.raw, plugins, tc.enabled)
			if !errors.Is(err, clihelpers.ErrUsage) {
				t.Fatalf("err = %v, want ErrUsage", err)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("err %q missing %q", err, tc.wantSub)
			}
		})
	}
}

// TestApplyFlags_PluginMethods pins the cobra flag → applyFlags wiring:
// the parsed map lands on initAnswers and PluginMethodsSet is true. Skip
// of the matching prompt is per-plugin (promptPluginMethodsForMulti
// short-circuits when picks[id] is already populated), not loop-level —
// promptForMissing always calls promptPluginMethodsForMulti once for the
// whole enabled list and lets the helper decide which ids to ask about.
func TestApplyFlags_PluginMethods(t *testing.T) {
	t.Parallel()
	plugins := methodFixturePlugins()
	flags := initFlags{
		Plugins:       "multi",
		PluginMethods: "multi=binary",
	}
	ans, err := applyFlags(&flags, plugins)
	if err != nil {
		t.Fatalf("applyFlags: %v", err)
	}
	if !ans.PluginMethodsSet {
		t.Errorf("PluginMethodsSet must be true after flag is supplied")
	}
	if ans.PluginMethods["multi"] != "binary" {
		t.Errorf("PluginMethods[multi] = %q, want %q", ans.PluginMethods["multi"], "binary")
	}
}

// TestApplyFlags_PluginMethodsRequiresEnabled cross-checks that
// --plugin-methods is rejected when --plugins has not selected the same id.
// applyFlags applies --plugins first, so the order in initFlags doesn't
// matter — the error comes from parsePluginMethods.
func TestApplyFlags_PluginMethodsRequiresEnabled(t *testing.T) {
	t.Parallel()
	plugins := methodFixturePlugins()
	flags := initFlags{
		Plugins:       "legacy",
		PluginMethods: "multi=binary",
	}
	_, err := applyFlags(&flags, plugins)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

// TestApplyDefaults_PluginMethodsEmpty pins the --yes branch: applyDefaults
// must mark PluginMethodsSet without inventing any picks, so plugins fall
// back to their DefaultMethod at install time.
func TestApplyDefaults_PluginMethodsEmpty(t *testing.T) {
	t.Parallel()
	plugins := methodFixturePlugins()
	ans, err := applyDefaults(initAnswers{ServiceName: "svc", Username: "dev"}, plugins)
	if err != nil {
		t.Fatal(err)
	}
	if !ans.PluginMethodsSet {
		t.Error("PluginMethodsSet must be true after applyDefaults")
	}
	if len(ans.PluginMethods) != 0 {
		t.Errorf("applyDefaults must not invent method picks, got %v", ans.PluginMethods)
	}
}

// TestRenderWorkspaceToml_WithMethods pins the active rendering path: a
// non-empty PluginMethods map becomes a real [plugins.methods] section
// with one `id = "method"` line per entry, alphabetically sorted.
func TestRenderWorkspaceToml_WithMethods(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "24.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
		Plugins:       []string{"multi", "legacy"},
		PluginMethods: map[string]string{"multi": "binary"},
	}, cat)
	want := "[plugins.methods]\nmulti = \"binary\"\n"
	if !strings.Contains(got, want) {
		t.Errorf("output missing %q\n--- got ---\n%s", want, got)
	}
	// The commented-out template must NOT appear when an active block is
	// emitted — that would mislead the user into copying the placeholder.
	if strings.Contains(got, "# <plugin-id> = \"<method-name>\"") {
		t.Errorf("active [plugins.methods] block must not also include the template placeholder\n--- got ---\n%s", got)
	}
}

// TestRenderWorkspaceToml_NoMethods_EmitsTemplate is the inverse: an empty
// PluginMethods leaves the commented template so the user discovers the
// section before they ever read the docs.
func TestRenderWorkspaceToml_NoMethods_EmitsTemplate(t *testing.T) {
	t.Parallel()
	for _, lang := range []i18n.Lang{i18n.LangEN, i18n.LangJA} {
		cat := i18n.New(lang)
		got := renderWorkspaceToml(containerSpec{
			ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "24.04",
			Shell: "bash", MountRoot: ".", Devcontainer: true,
		}, cat)
		if !strings.Contains(got, "# [plugins.methods]") {
			t.Errorf("[%s] empty methods should emit commented template; got:\n%s", lang, got)
		}
		// And the active form must not have leaked in.
		if strings.Contains(got, "\n[plugins.methods]") {
			t.Errorf("[%s] active [plugins.methods] header must not appear when picks is empty", lang)
		}
	}
}

// TestRenderWorkspaceToml_MethodsSortedDeterministically guards the
// alphabetical-by-id contract that writePluginMethods promises. The map
// iteration order in Go is non-deterministic, so this test fails fast if
// someone replaces the sort with `for id := range picks`.
func TestRenderWorkspaceToml_MethodsSortedDeterministically(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	got := renderWorkspaceToml(containerSpec{
		ServiceName: "svc", Username: "dev", Image: "ubuntu", ImageVersion: "24.04",
		Shell: "bash", MountRoot: ".", Devcontainer: true,
		Plugins: []string{"a", "b", "c"},
		PluginMethods: map[string]string{
			"c": "x", "a": "y", "b": "z",
		},
	}, cat)
	wantBlock := "[plugins.methods]\na = \"y\"\nb = \"z\"\nc = \"x\"\n"
	if !strings.Contains(got, wantBlock) {
		t.Errorf("methods must be sorted by id; want block:\n%s\n--- got ---\n%s", wantBlock, got)
	}
}

func TestI18n_PluginMethodKeysDefinedBothLocales(t *testing.T) {
	t.Parallel()
	keys := []string{
		"init_prompt_plugin_method",
		"init_desc_plugin_method",
		"init_toml_section_plugins_methods",
		"init_toml_template_plugins_methods",
	}
	for _, lang := range []i18n.Lang{i18n.LangEN, i18n.LangJA} {
		cat := i18n.New(lang)
		for _, k := range keys {
			msg := cat.Msg(k)
			if msg == "" {
				t.Errorf("[%s] catalog missing %q", lang, k)
				continue
			}
			// Bare key fallback means the entry is undefined — Msg returns
			// the key itself, which would surface in the UI as gibberish.
			if msg == k {
				t.Errorf("[%s] %q resolves to the bare key (undefined entry)", lang, k)
			}
		}
	}
}

// TestPromptOnePluginMethod_AcceptsDefault drives promptOnePluginMethod with
// a no-op form runner — the accessible equivalent of the user pressing Enter
// on the pre-selected DefaultMethod — and confirms it returns that default.
//
//nolint:paralleltest // withFakePrompt swaps the shared runSingleFieldForm seam.
func TestPromptOnePluginMethod_AcceptsDefault(t *testing.T) {
	withFakePrompt(t, func(huh.Field) error { return nil })
	cat := i18n.New(i18n.LangEN)
	p := methodFixturePlugins()["multi"] // DefaultMethod = "official"

	got, err := promptOnePluginMethod(cat, "multi", p)
	if err != nil {
		t.Fatalf("promptOnePluginMethod err = %v", err)
	}
	if got != "official" {
		t.Errorf("picked = %q, want %q (the pre-selected DefaultMethod)", got, "official")
	}
}

// TestPromptOnePluginMethod_PropagatesFormError pins the (value, error)
// contract: a form failure surfaces verbatim with an empty pick. It also
// confirms the seam is handed a non-nil field to run.
//
//nolint:paralleltest // withFakePrompt swaps the shared runSingleFieldForm seam.
func TestPromptOnePluginMethod_PropagatesFormError(t *testing.T) {
	sentinel := errors.New("form blew up")
	var gotField huh.Field
	withFakePrompt(t, func(f huh.Field) error { gotField = f; return sentinel })
	cat := i18n.New(i18n.LangEN)
	p := methodFixturePlugins()["multi"]

	got, err := promptOnePluginMethod(cat, "multi", p)
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want the form error", err)
	}
	if got != "" {
		t.Errorf("picked = %q, want empty on error", got)
	}
	if gotField == nil {
		t.Error("runSingleFieldForm was handed a nil field")
	}
}
