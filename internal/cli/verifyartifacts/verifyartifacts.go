// Package verifyartifactscli implements `wsd verify-artifacts`.
//
// It loads a workspace.toml fixture and the four generated artifacts
// (Dockerfile, docker-compose.yml, .devcontainer/devcontainer.json, and the
// per-shell rc fragment under config/) and asserts that every option declared in
// the fixture is reflected in the corresponding artifact. This protects CI
// against silent regressions where a generator stops emitting a section
// (e.g. someone refactors the compose generator and accidentally drops
// [container.resources] support).
//
// Replaces the legacy tests/integration/verify_generated_artifacts.py.
package verifyartifactscli

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/sukekyo26/cocoon/internal/config"
)

// ErrUsage is returned for invalid CLI invocation. Maps to exit code 2.
var ErrUsage = errors.New("usage error")

// ErrFailure is returned when one or more verification assertions fail.
// Maps to exit code 1.
var ErrFailure = errors.New("failure")

var jsoncLineComment = regexp.MustCompile(`(?m)^\s*//.*$`)

func stripJSONC(b []byte) []byte {
	return []byte(jsoncLineComment.ReplaceAllString(string(b), ""))
}

type verifier struct {
	ws       *config.Workspace
	df       string
	compose  map[string]any
	devc     map[string]any
	bashrc   string
	rcSyntax string
	failures []string
}

func (v *verifier) fail(format string, args ...any) {
	v.failures = append(v.failures, fmt.Sprintf(format, args...))
}

// mainService returns the main dev-container compose service.
func (v *verifier) mainService() map[string]any {
	services, _ := v.compose["services"].(map[string]any) //nolint:errcheck // type assert ok-pattern.
	name := v.ws.Container.ServiceName
	if name == "" {
		name = "dev"
	}
	svc, ok := services[name].(map[string]any)
	if !ok {
		v.fail("compose: main service %q not found", name)
		return map[string]any{}
	}
	return svc
}

func (v *verifier) run() {
	svc := v.mainService()
	v.verifyResources(svc)
	v.verifyPorts(svc)
	v.verifyVolumesAndMounts(svc)
	v.verifyEnv(svc)
	v.verifyApt()
	v.verifyLocale(svc)
	v.verifyGit()
	v.verifyShell()
	v.verifyDockerfileHooks()
	v.verifyDevcontainerFeaturesAndExtensions()
	v.verifySidecars()
	v.verifyPinnedVersions()
}

//nolint:gocyclo // straight-line series of independent field checks; splitting hurts readability.
func (v *verifier) verifyResources(svc map[string]any) {
	res := v.ws.Container.Resources
	if res == nil {
		return
	}
	if res.ShmSize != nil && asString(svc["shm_size"]) != *res.ShmSize {
		v.fail("compose.shm_size: expected %q, got %q", *res.ShmSize, asString(svc["shm_size"]))
	}
	if res.PidsLimit != nil && asInt(svc["pids_limit"]) != *res.PidsLimit {
		v.fail("compose.pids_limit: expected %d, got %v", *res.PidsLimit, svc["pids_limit"])
	}
	if res.StopGracePeriod != nil && asString(svc["stop_grace_period"]) != *res.StopGracePeriod {
		v.fail("compose.stop_grace_period: expected %q, got %q", *res.StopGracePeriod, asString(svc["stop_grace_period"]))
	}
	if res.CPUs != nil && asFloat(svc["cpus"]) != *res.CPUs {
		v.fail("compose.cpus: expected %v, got %v", *res.CPUs, svc["cpus"])
	}
	if res.Memory != nil && asString(svc["mem_limit"]) != *res.Memory {
		v.fail("compose.mem_limit (from memory): expected %q, got %q", *res.Memory, asString(svc["mem_limit"]))
	}
	ulimits, _ := svc["ulimits"].(map[string]any)   //nolint:errcheck // type assert ok-pattern.
	nofile, _ := ulimits["nofile"].(map[string]any) //nolint:errcheck // type assert ok-pattern.
	if res.NofileSoft != nil && asInt(nofile["soft"]) != *res.NofileSoft {
		v.fail("compose.ulimits.nofile.soft: expected %d, got %v", *res.NofileSoft, nofile["soft"])
	}
	if res.NofileHard != nil && asInt(nofile["hard"]) != *res.NofileHard {
		v.fail("compose.ulimits.nofile.hard: expected %d, got %v", *res.NofileHard, nofile["hard"])
	}
}

func (v *verifier) verifyPorts(svc map[string]any) {
	var forward []any
	if v.ws.Ports != nil {
		forward = v.ws.Ports.Forward
	}
	composeEntries, _ := svc["ports"].([]any) //nolint:errcheck // type assert ok-pattern.
	for i, p := range config.ComposePortEntries(forward) {
		if !composeHasPort(composeEntries, p) {
			v.fail("compose.ports[%d]: missing entry for %v (got %v)", i, p, composeEntries)
		}
	}

	expected := config.DevcontainerPortEntries(forward, nil)
	if v.ws.Ports == nil {
		expected = []int{3000}
	}
	var extras []int
	if dev := mapAt(v.ws.Devcontainer, "forwardPorts"); dev != nil {
		extras = anySliceToInts(dev)
	}
	expectedUnion := uniqueInts(append(append([]int{}, expected...), extras...))
	actualFwd := anySliceToInts(v.devc["forwardPorts"])
	if !sameInts(actualFwd, expectedUnion) {
		v.fail("devcontainer.forwardPorts: expected %v, got %v", expectedUnion, actualFwd)
	}
}

// composeHasPort reports whether one of the parsed compose ports entries
// matches the expected ComposePort. Short form must be exactly equal; long
// form is matched on the explicit keys (other compose entries with extra
// keys still satisfy the expectation).
func composeHasPort(entries []any, want config.ComposePort) bool {
	for _, raw := range entries {
		if !want.IsLong() {
			if s, ok := raw.(string); ok && s == want.Short {
				return true
			}
			continue
		}
		got, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		match := true
		for k, vWant := range want.Long {
			vGot, has := got[k]
			if !has || fmt.Sprintf("%v", vGot) != fmt.Sprintf("%v", vWant) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func (v *verifier) verifyVolumesAndMounts(svc map[string]any) {
	composeVols, _ := v.compose["volumes"].(map[string]any) //nolint:errcheck // type assert ok-pattern.
	svcVols := stringSlice(svc["volumes"])
	for name, path := range v.ws.Volumes {
		if _, ok := composeVols[name]; !ok {
			v.fail("compose.volumes: missing top-level entry %q", name)
		}
		expectedMount := fmt.Sprintf("%s:%s", name, path)
		if !contains(svcVols, expectedMount) {
			v.fail("compose.services.<svc>.volumes: missing %q", expectedMount)
		}
	}
	for _, m := range v.ws.Mounts {
		suffix := ""
		if m.Readonly {
			suffix = ":ro"
		}
		expected := fmt.Sprintf("%s:%s%s", m.Source, m.Target, suffix)
		if !contains(svcVols, expected) {
			v.fail("compose bind mount missing: %q", expected)
		}
	}
}

func (v *verifier) verifyEnv(svc map[string]any) {
	envList := envAsList(svc["environment"])
	expected := map[string]string{}
	for k, val := range v.ws.Env {
		expected[k] = val
	}
	if v.ws.Locale != nil && v.ws.Locale.Timezone != nil {
		expected["TZ"] = *v.ws.Locale.Timezone
	}
	for k, val := range expected {
		entry := fmt.Sprintf("%s=%s", k, val)
		if !contains(envList, entry) {
			v.fail("compose env: missing %q (got %v)", entry, envList)
		}
	}
}

func (v *verifier) verifyApt() {
	if v.ws.Apt == nil {
		return
	}
	for _, pkg := range v.ws.Apt.Packages {
		// Each extra package is emitted on its own line as `    <pkg> \`.
		pat := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(pkg) + `\s*\\\s*$`)
		if !pat.MatchString(v.df) {
			v.fail("Dockerfile: apt package %q not found in install block", pkg)
		}
	}
}

func (v *verifier) verifyLocale(svc map[string]any) {
	if v.ws.Locale == nil {
		return
	}
	if v.ws.Locale.Lang != nil {
		lang := *v.ws.Locale.Lang
		if !strings.Contains(v.df, fmt.Sprintf("locale-gen en_US.UTF-8 %s", lang)) &&
			!strings.Contains(v.df, fmt.Sprintf("locale-gen %s", lang)) {
			v.fail("Dockerfile: locale-gen line for %q not found", lang)
		}
		if !strings.Contains(v.df, fmt.Sprintf("ENV LANG=%s", lang)) {
			v.fail("Dockerfile: ENV LANG=%s not found", lang)
		}
		if !strings.Contains(v.df, fmt.Sprintf("ENV LC_ALL=%s", lang)) {
			v.fail("Dockerfile: ENV LC_ALL=%s not found", lang)
		}
	}
	if v.ws.Locale.Timezone != nil {
		envList := envAsList(svc["environment"])
		want := fmt.Sprintf("TZ=%s", *v.ws.Locale.Timezone)
		if !contains(envList, want) {
			v.fail("compose env: %s not found", want)
		}
	}
}

func (v *verifier) verifyGit() {
	if v.ws.Git == nil {
		return
	}
	if v.ws.Git.UserName != nil {
		name := *v.ws.Git.UserName
		quoted := fmt.Sprintf("git config --system user.name  '%s'", name)
		bare := fmt.Sprintf("git config --system user.name  %s", name)
		if !strings.Contains(v.df, quoted) && !strings.Contains(v.df, bare) {
			v.fail("Dockerfile: git user.name %q not configured", name)
		}
	}
	if v.ws.Git.UserEmail != nil {
		email := *v.ws.Git.UserEmail
		want := fmt.Sprintf("git config --system user.email %s", email)
		if !strings.Contains(v.df, want) {
			v.fail("Dockerfile: git user.email %q not configured", email)
		}
	}
}

func (v *verifier) verifyShell() {
	if v.bashrc == "" || v.ws.Container.Shell == nil {
		return
	}
	aliasPattern := `(?m)^alias %s=`
	envPattern := `(?m)^export %s=`
	if v.rcSyntax == "fish" {
		aliasPattern = `(?m)^alias %s `
		envPattern = `(?m)^set -gx %s `
	}
	for k, val := range v.ws.Container.Shell.Aliases {
		alias := regexp.MustCompile(fmt.Sprintf(aliasPattern, regexp.QuoteMeta(k)))
		if !alias.MatchString(v.bashrc) {
			v.fail("shellrc fragment: alias %q missing", k)
		} else if !strings.Contains(v.bashrc, val) {
			v.fail("shellrc fragment: alias %q value %q missing", k, val)
		}
	}
	for k, val := range v.ws.Container.Shell.Env {
		exp := regexp.MustCompile(fmt.Sprintf(envPattern, regexp.QuoteMeta(k)))
		if !exp.MatchString(v.bashrc) {
			v.fail("shellrc fragment: export/set %q missing", k)
		} else if !strings.Contains(v.bashrc, val) {
			v.fail("shellrc fragment: export/set %q value %q missing", k, val)
		}
	}
}

func (v *verifier) verifyDockerfileHooks() {
	if v.ws.Dockerfile == nil {
		return
	}
	if v.ws.Dockerfile.PreUserSetup != nil {
		body := strings.TrimSpace(*v.ws.Dockerfile.PreUserSetup)
		if body != "" && !strings.Contains(v.df, body) {
			v.fail("Dockerfile: [dockerfile].pre_user_setup content not found")
		}
	}
	if v.ws.Dockerfile.PostPlugins != nil {
		body := strings.TrimSpace(*v.ws.Dockerfile.PostPlugins)
		if body != "" && !strings.Contains(v.df, body) {
			v.fail("Dockerfile: [dockerfile].post_plugins content not found")
		}
	}
}

func (v *verifier) verifyDevcontainerFeaturesAndExtensions() {
	expectedFeats, _ := mapAt(v.ws.Devcontainer, "features").(map[string]any) //nolint:errcheck // type assert ok-pattern.
	actualFeats, _ := v.devc["features"].(map[string]any)                     //nolint:errcheck // type assert ok-pattern.
	for fid, cfg := range expectedFeats {
		got, ok := actualFeats[fid]
		if !ok {
			v.fail("devcontainer.features: missing %q", fid)
			continue
		}
		if !deepEqualJSON(cfg, got) {
			v.fail("devcontainer.features[%q]: expected %v, got %v", fid, cfg, got)
		}
	}
	//nolint:errcheck // type assert ok-pattern.
	customizations, _ := mapAt(v.ws.Devcontainer, "customizations").(map[string]any)
	vscode, _ := customizations["vscode"].(map[string]any) //nolint:errcheck // type assert ok-pattern.
	expectedExts := anySliceToStrings(vscode["extensions"])

	actualCust, _ := v.devc["customizations"].(map[string]any) //nolint:errcheck // type assert ok-pattern.
	actualVscode, _ := actualCust["vscode"].(map[string]any)   //nolint:errcheck // type assert ok-pattern.
	actualExts := anySliceToStrings(actualVscode["extensions"])
	for _, ext := range expectedExts {
		if !contains(actualExts, ext) {
			v.fail("devcontainer.customizations.vscode.extensions: missing %q", ext)
		}
	}
}

//nolint:gocognit,gocyclo // mirrors per-field sidecar layout; splitting would scatter related checks.
func (v *verifier) verifySidecars() {
	if len(v.ws.Services) == 0 {
		return
	}
	composeServices, _ := v.compose["services"].(map[string]any) //nolint:errcheck // type assert ok-pattern.
	composeVols, _ := v.compose["volumes"].(map[string]any)      //nolint:errcheck // type assert ok-pattern.

	for name, spec := range v.ws.Services {
		raw, ok := composeServices[name].(map[string]any)
		if !ok {
			v.fail("compose: sidecar service %q not found", name)
			continue
		}
		if spec.Image != "" && asString(raw["image"]) != spec.Image {
			v.fail("compose.%s.image: expected %q, got %q", name, spec.Image, asString(raw["image"]))
		}
		actualPorts := anySliceToStrings(raw["ports"])
		for _, p := range spec.Ports {
			ps := fmt.Sprintf("%v", p)
			if !contains(actualPorts, ps) {
				v.fail("compose.%s.ports: missing %q", name, ps)
			}
		}
		envList := envAsList(raw["environment"])
		for k, val := range spec.Env {
			entry := fmt.Sprintf("%s=%s", k, val)
			if !contains(envList, entry) {
				v.fail("compose.%s.environment: missing %q", name, entry)
			}
		}
		scVols := stringSlice(raw["volumes"])
		for volKey, volPath := range spec.Volumes {
			ns := fmt.Sprintf("%s_%s", name, volKey)
			expected := fmt.Sprintf("%s:%s", ns, volPath)
			if !contains(scVols, expected) {
				v.fail("compose.%s.volumes: missing %q", name, expected)
			}
			if _, ok := composeVols[ns]; !ok {
				v.fail("compose.volumes: missing top-level entry %q for sidecar %q", ns, name)
			}
		}
		if len(spec.Healthcheck) > 0 {
			if _, ok := raw["healthcheck"]; !ok {
				v.fail("compose.%s: missing healthcheck", name)
			}
		}
		if spec.Restart != nil && asString(raw["restart"]) != string(*spec.Restart) {
			v.fail("compose.%s.restart: expected %q, got %q", name, *spec.Restart, asString(raw["restart"]))
		}
		actualDeps := anySliceToStrings(raw["depends_on"])
		for _, dep := range spec.DependsOn {
			if !contains(actualDeps, dep) {
				v.fail("compose.%s.depends_on: missing %q", name, dep)
			}
		}
	}
}

func (v *verifier) verifyPinnedVersions() {
	for plugin, spec := range v.ws.Plugins.Versions {
		if spec.Pin == "" {
			continue
		}
		if !strings.Contains(v.df, spec.Pin) {
			v.fail("Dockerfile: pinned version %q for plugin %q not found", spec.Pin, plugin)
		}
	}
}

// ---------- helpers ----------

func mapAt(m map[string]any, key string) any {
	if m == nil {
		return nil
	}
	return m[key]
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func stringSlice(v any) []string {
	out := []string{}
	if v == nil {
		return out
	}
	if s, ok := v.([]any); ok {
		for _, e := range s {
			out = append(out, fmt.Sprintf("%v", e))
		}
	}
	return out
}

func anySliceToStrings(v any) []string {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		out := make([]string, 0, len(s))
		for _, e := range s {
			out = append(out, fmt.Sprintf("%v", e))
		}
		return out
	}
	return nil
}

func anySliceToInts(v any) []int {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		out := make([]int, 0, len(s))
		for _, e := range s {
			switch n := e.(type) {
			case int:
				out = append(out, n)
			case int64:
				out = append(out, int(n))
			case float64:
				out = append(out, int(n))
			}
		}
		return out
	}
	return nil
}

func uniqueInts(in []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(in))
	for _, n := range in {
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	return out
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

// envAsList returns docker-compose `environment` entries as a list of "K=V"
// strings regardless of whether YAML parsed them as a list or a mapping.
func envAsList(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			out = append(out, fmt.Sprintf("%v", e))
		}
		return out
	case map[string]any:
		out := make([]string, 0, len(t))
		for k, val := range t {
			out = append(out, fmt.Sprintf("%s=%v", k, val))
		}
		return out
	default:
		return nil
	}
}

func deepEqualJSON(a, b any) bool {
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(ab) == string(bb)
}
