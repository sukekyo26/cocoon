//nolint:testpackage // exercises unexported prompt / form-builder helpers.
package initcli

import (
	"errors"
	"testing"

	"github.com/charmbracelet/huh"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// TestPluginsMultiSelect_BuildsForEveryExcludeID is a smoke test: huh's
// option list isn't reachable through a stable API, so this only confirms
// construction does not panic. Exclusion behavior itself is covered by
// TestFilterPluginIDs.
func TestPluginsMultiSelect_BuildsForEveryExcludeID(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	cat := i18n.New(i18n.LangEN)

	for _, excludeID := range []string{"", "rust", "go", "node", "deno"} {
		excludeID := excludeID
		t.Run("exclude="+excludeID, func(t *testing.T) {
			t.Parallel()
			// Fresh slice per subtest: parallel subtests must not share the
			// pointer handed to huh's MultiSelect.Value.
			var target []string
			sel := pluginsMultiSelect(cat, plugins, excludeID, &target)
			if sel == nil {
				t.Fatal("pluginsMultiSelect returned nil")
			}
		})
	}
}

// withFakePrompt swaps the package-level runSingleFieldForm seam for the
// duration of the test and restores it via t.Cleanup. fake replaces the
// real huh.Form.Run, so tests can exercise prompt-orchestrator branching
// logic (when does the orchestrator decide to prompt? what does it do on
// error?) without a real TTY.
//
// All callers must be //nolint:paralleltest because the seam is shared
// package state.
func withFakePrompt(t *testing.T, fake func(huh.Field) error) {
	t.Helper()
	orig := runSingleFieldForm
	t.Cleanup(func() { runSingleFieldForm = orig })
	runSingleFieldForm = fake
}

// countingPrompt returns a fake that counts how many times it is invoked
// and a pointer the test can read after the call. Useful for "when does
// the orchestrator decide to prompt?" assertions.
func countingPrompt(t *testing.T) (fake func(huh.Field) error, calls *int) {
	t.Helper()
	n := 0
	return func(huh.Field) error { n++; return nil }, &n
}

// fullyPopulatedAnswers returns an initAnswers with every *Set flag flipped
// to true so no prompt-orchestrator branch fires. Tests use this as a
// baseline and selectively clear fields to verify "this missing field
// triggers exactly N prompt(s)".
func fullyPopulatedAnswers() initAnswers {
	return initAnswers{
		ServiceName:       "svc",
		Username:          "alice",
		Image:             "ubuntu",
		ImageSet:          true,
		ImageVersion:      "22.04",
		ImageVersionSet:   true,
		Shell:             "bash",
		ShellSet:          true,
		MountRoot:         ".",
		MountRootSet:      true,
		Dir:               "workspace",
		DirSet:            true,
		Devcontainer:      true,
		DevcontainerSet:   true,
		Certificates:      false,
		CertificatesSet:   true,
		Sudo:              config.SudoModeNoPasswd,
		SudoSet:           true,
		AptCategories:     nil,
		AptSet:            true,
		Plugins:           nil,
		PluginsSet:        true,
		PluginVersions:    map[string]string{},
		PluginVersionsSet: true,
		PluginMethods:     map[string]string{},
		PluginMethodsSet:  true,
		AliasBundles:      nil,
		AliasBundlesSet:   true,
		Ports:             nil,
		PortsSet:          true,
	}
}

// TestPromptIdentityAndImage_AllPresetSkipsEveryPrompt pins the contract
// that a fully populated initAnswers (after applyFlags / applyDefaults)
// never reopens a prompt — re-running `cocoon init` after a fully-flagged
// invocation must be a no-op for the identity/image group.
//
//nolint:paralleltest // mutates the package-level runSingleFieldForm seam
func TestPromptIdentityAndImage_AllPresetSkipsEveryPrompt(t *testing.T) {
	fake, calls := countingPrompt(t)
	withFakePrompt(t, fake)

	ans := fullyPopulatedAnswers()
	if err := promptIdentityAndImage(&ans, i18n.New(i18n.LangEN)); err != nil {
		t.Fatalf("promptIdentityAndImage: %v", err)
	}
	if *calls != 0 {
		t.Errorf("calls = %d, want 0 (all fields preset, no prompts should fire)", *calls)
	}
}

// TestPromptIdentityAndImage_PromptsForEachMissing covers each branch of
// the identity/image orchestrator. The table clears one *Set / value at a
// time and asserts the prompt count matches what the branch claims to
// guard.
//
//nolint:paralleltest // mutates the package-level runSingleFieldForm seam
func TestPromptIdentityAndImage_PromptsForEachMissing(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*initAnswers)
		wantCalls int
	}{
		{
			name:      "service_name_missing",
			mutate:    func(a *initAnswers) { a.ServiceName = "" },
			wantCalls: 1,
		},
		{
			name:      "username_missing",
			mutate:    func(a *initAnswers) { a.Username = "" },
			wantCalls: 1,
		},
		{
			name:      "image_not_set",
			mutate:    func(a *initAnswers) { a.ImageSet = false },
			wantCalls: 1,
		},
		{
			name:      "image_version_not_set",
			mutate:    func(a *initAnswers) { a.ImageVersionSet = false },
			wantCalls: 1,
		},
		{
			name:      "shell_not_set",
			mutate:    func(a *initAnswers) { a.ShellSet = false },
			wantCalls: 1,
		},
		{
			name: "everything_missing",
			mutate: func(a *initAnswers) {
				a.ServiceName = ""
				a.Username = ""
				a.ImageSet = false
				a.ImageVersionSet = false
				a.ShellSet = false
			},
			wantCalls: 5,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake, calls := countingPrompt(t)
			withFakePrompt(t, fake)

			ans := fullyPopulatedAnswers()
			tc.mutate(&ans)
			if err := promptIdentityAndImage(&ans, i18n.New(i18n.LangEN)); err != nil {
				t.Fatalf("promptIdentityAndImage: %v", err)
			}
			if *calls != tc.wantCalls {
				t.Errorf("calls = %d, want %d", *calls, tc.wantCalls)
			}
		})
	}
}

// TestPromptIdentityAndImage_ImageDefaultsToDebian pins the behavior that
// promptIdentityAndImage applies the "debian" default to ans.Image when it
// is empty, before the image prompt fires. This guards against a refactor
// silently dropping the default.
//
//nolint:paralleltest // mutates the package-level runSingleFieldForm seam
func TestPromptIdentityAndImage_ImageDefaultsToDebian(t *testing.T) {
	fake, _ := countingPrompt(t)
	withFakePrompt(t, fake)

	ans := fullyPopulatedAnswers()
	ans.ImageSet = false
	ans.Image = "" // force the default-application branch

	if err := promptIdentityAndImage(&ans, i18n.New(i18n.LangEN)); err != nil {
		t.Fatalf("promptIdentityAndImage: %v", err)
	}
	if ans.Image != "debian" {
		t.Errorf("Image = %q, want %q (default applied before prompt)", ans.Image, "debian")
	}
	if !ans.ImageSet {
		t.Error("ImageSet must be true after the orchestrator runs the image prompt")
	}
}

// TestPromptIdentityAndImage_PropagatesPromptError pins that a prompt
// error short-circuits the orchestrator — the next field is not
// attempted. The fake returns the failure on the first call so the
// counter caps at 1.
//
//nolint:paralleltest // mutates the package-level runSingleFieldForm seam
func TestPromptIdentityAndImage_PropagatesPromptError(t *testing.T) {
	wantErr := errors.New("prompt boom")
	calls := 0
	withFakePrompt(t, func(huh.Field) error {
		calls++
		return wantErr
	})

	ans := fullyPopulatedAnswers()
	ans.ServiceName = "" // forces the first prompt
	ans.Username = ""    // would force a second prompt — must not run

	err := promptIdentityAndImage(&ans, i18n.New(i18n.LangEN))
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want errors.Is %v", err, wantErr)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (short-circuit on first error)", calls)
	}
}

// TestPromptWorkspaceOptions_AllPresetSkipsEveryPrompt is the workspace-
// options equivalent of TestPromptIdentityAndImage_AllPresetSkipsEveryPrompt.
//
//nolint:paralleltest // mutates the package-level runSingleFieldForm seam
func TestPromptWorkspaceOptions_AllPresetSkipsEveryPrompt(t *testing.T) {
	fake, calls := countingPrompt(t)
	withFakePrompt(t, fake)

	ans := fullyPopulatedAnswers()
	if err := promptWorkspaceOptions(&ans, i18n.New(i18n.LangEN)); err != nil {
		t.Fatalf("promptWorkspaceOptions: %v", err)
	}
	if *calls != 0 {
		t.Errorf("calls = %d, want 0", *calls)
	}
}

// TestPromptWorkspaceOptions_PromptsForEachMissing covers each branch.
//
//nolint:paralleltest // mutates the package-level runSingleFieldForm seam
func TestPromptWorkspaceOptions_PromptsForEachMissing(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*initAnswers)
		wantCalls int
	}{
		{"alias_bundles_not_set", func(a *initAnswers) { a.AliasBundlesSet = false }, 1},
		{"mount_root_not_set", func(a *initAnswers) { a.MountRootSet = false }, 1},
		{"dir_not_set", func(a *initAnswers) { a.DirSet = false }, 1},
		{"devcontainer_not_set", func(a *initAnswers) { a.DevcontainerSet = false }, 1},
		{"certificates_not_set", func(a *initAnswers) { a.CertificatesSet = false }, 1},
		{"sudo_not_set", func(a *initAnswers) { a.SudoSet = false }, 1},
		// password mode with SudoSet=true skips the Select but still prompts
		// for the password (SudoPassword empty) — exercises that second prompt.
		{"sudo_password_needs_pw", func(a *initAnswers) { a.Sudo = config.SudoModePassword }, 1},
		{"ports_not_set", func(a *initAnswers) { a.PortsSet = false }, 1},
		{"apt_not_set", func(a *initAnswers) { a.AptSet = false }, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake, calls := countingPrompt(t)
			withFakePrompt(t, fake)

			ans := fullyPopulatedAnswers()
			tc.mutate(&ans)
			if err := promptWorkspaceOptions(&ans, i18n.New(i18n.LangEN)); err != nil {
				t.Fatalf("promptWorkspaceOptions: %v", err)
			}
			if *calls != tc.wantCalls {
				t.Errorf("calls = %d, want %d", *calls, tc.wantCalls)
			}
		})
	}
}

// TestPromptDir_AppliesWorkspaceDefaultOnBlank pins the doc claim that a
// blank input falls back to "workspace" — the renderer relies on this so
// init never emits an empty dir field.
//
//nolint:paralleltest // mutates the package-level runSingleFieldForm seam
func TestPromptDir_AppliesWorkspaceDefaultOnBlank(t *testing.T) {
	// fake returns nil without touching the target — the raw var inside
	// promptDir therefore stays "" and the default-application branch
	// must rewrite it to "workspace".
	withFakePrompt(t, func(huh.Field) error { return nil })

	ans := fullyPopulatedAnswers()
	ans.DirSet = false
	ans.Dir = ""

	if err := promptDir(&ans, i18n.New(i18n.LangEN)); err != nil {
		t.Fatalf("promptDir: %v", err)
	}
	if ans.Dir != "workspace" {
		t.Errorf("Dir = %q, want %q (blank-input default)", ans.Dir, "workspace")
	}
	if !ans.DirSet {
		t.Error("DirSet must be true after promptDir runs")
	}
}

// TestPromptPluginsWithRetry_SucceedsOnFirstAttempt covers the no-conflict
// happy path: target is non-conflicting on entry, fake returns nil, loop
// returns after a single attempt.
//
//nolint:paralleltest // mutates the package-level runSingleFieldForm seam
func TestPromptPluginsWithRetry_SucceedsOnFirstAttempt(t *testing.T) {
	plugins := map[string]*plugin.Plugin{
		"alpha": {Metadata: plugin.Metadata{Name: "Alpha"}},
		"beta":  {Metadata: plugin.Metadata{Name: "Beta"}},
	}
	fake, calls := countingPrompt(t)
	withFakePrompt(t, fake)

	target := []string{"alpha"} // disjoint — no conflict
	if err := promptPluginsWithRetry(i18n.New(i18n.LangEN), plugins, "", &target); err != nil {
		t.Fatalf("promptPluginsWithRetry: %v", err)
	}
	if *calls != 1 {
		t.Errorf("calls = %d, want 1", *calls)
	}
}

// TestPromptPluginsWithRetry_GivesUpAfterMaxAttempts pins the retry
// contract: three consecutive conflict detections return ErrUsage so
// scripted invocations cannot loop forever. The conflict is preserved
// across loop iterations because the fake doesn't mutate target.
//
//nolint:paralleltest // mutates the package-level runSingleFieldForm seam
func TestPromptPluginsWithRetry_GivesUpAfterMaxAttempts(t *testing.T) {
	plugins := map[string]*plugin.Plugin{
		"alpha": {Metadata: plugin.Metadata{Name: "Alpha", Conflicts: []string{"beta"}}},
		"beta":  {Metadata: plugin.Metadata{Name: "Beta", Conflicts: []string{"alpha"}}},
	}
	fake, calls := countingPrompt(t)
	withFakePrompt(t, fake)

	target := []string{"alpha", "beta"} // conflicting pair
	err := promptPluginsWithRetry(i18n.New(i18n.LangEN), plugins, "", &target)
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Fatalf("err = %v, want errors.Is ErrUsage", err)
	}
	if *calls != 3 {
		t.Errorf("calls = %d, want 3 (maxAttempts in promptPluginsWithRetry)", *calls)
	}
}

// TestPromptPluginsWithRetry_PropagatesPromptError pins that an error
// from the prompt itself (not a conflict) short-circuits the loop on
// the first iteration.
//
//nolint:paralleltest // mutates the package-level runSingleFieldForm seam
func TestPromptPluginsWithRetry_PropagatesPromptError(t *testing.T) {
	wantErr := errors.New("prompt boom")
	calls := 0
	withFakePrompt(t, func(huh.Field) error {
		calls++
		return wantErr
	})

	plugins := map[string]*plugin.Plugin{"alpha": {Metadata: plugin.Metadata{Name: "Alpha"}}}
	target := []string{"alpha"}
	err := promptPluginsWithRetry(i18n.New(i18n.LangEN), plugins, "", &target)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want errors.Is %v", err, wantErr)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on prompt error)", calls)
	}
}

// TestPromptForMissing_RunsAllThreeOrchestrators verifies the sequencing
// claim in promptForMissing's docstring: identity → workspace options →
// plugin selection. With every field unset, every prompt fires once; an
// orchestrator that silently skipped its stage would leave the count
// short.
//
//nolint:paralleltest // mutates the package-level runSingleFieldForm seam
func TestPromptForMissing_RunsAllThreeOrchestrators(t *testing.T) {
	plugins := map[string]*plugin.Plugin{
		"alpha": {Metadata: plugin.Metadata{Name: "Alpha"}},
	}
	fake, calls := countingPrompt(t)
	withFakePrompt(t, fake)

	// initAnswers zero value — every *Set is false, every value is empty.
	_, err := promptForMissing(initAnswers{}, i18n.New(i18n.LangEN), plugins)
	if err != nil {
		t.Fatalf("promptForMissing: %v", err)
	}
	// 5 identity/image + 8 workspace + 1 plugin multiselect = 14.
	// (Plugin method/version prompts are gated on version_capable plugins
	// and methods present, both absent in our synthetic catalog, so the
	// inner per-plugin loops skip with 0 calls.)
	const want = 14
	if *calls != want {
		t.Errorf("calls = %d, want %d (identity 5 + workspace 8 + plugins 1)", *calls, want)
	}
}
