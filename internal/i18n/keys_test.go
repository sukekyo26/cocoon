//nolint:testpackage // accesses the private `messages` map to assert catalog parity.
package i18n

import (
	"sort"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/warn"
)

// TestUserFacingKeyParityInBothLanguages pins the contract that every CLI
// help key (cmd_*, flag_*, help_*, usage_*, version_*) and init-prompt key
// (init_*) exists in both the English and Japanese tables.
//
// Catalog.Msg falls back to English when a key is missing in the active
// language and falls back to the key string itself when missing in both
// — without this assertion a misnamed ja entry would silently surface
// the English text (or worse, the raw key name) at runtime.
func TestUserFacingKeyParityInBothLanguages(t *testing.T) {
	t.Parallel()
	en := messages[LangEN]
	ja := messages[LangJA]
	if en == nil || ja == nil {
		t.Fatalf("expected both language tables to be populated; en=%v ja=%v", en != nil, ja != nil)
	}
	missingInJA := diffPrefixedKeys(en, ja)
	missingInEN := diffPrefixedKeys(ja, en)
	if len(missingInJA) > 0 {
		t.Errorf("user-facing keys missing in ja table: %v", missingInJA)
	}
	if len(missingInEN) > 0 {
		t.Errorf("user-facing keys missing in en table: %v", missingInEN)
	}
}

// diffPrefixedKeys returns keys present in src but not in dst, filtered
// to the CLI help and init-prompt namespaces so unrelated tables (clean /
// rebuild / plugin prompts) are not flagged.
func diffPrefixedKeys(src, dst map[string]string) []string {
	prefixes := []string{
		"cmd_", "flag_", "help_", "usage_", "version_", "init_",
		"gen_", "lock_", "selfupdate_", "plugin_", "clean_", "rebuild_",
		"docker_", "warn_", "info_", "err_",
	}
	var missing []string
	for k := range src {
		if !hasAnyPrefix(k, prefixes) {
			continue
		}
		if _, ok := dst[k]; !ok {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing
}

// TestWarnCodesHaveCatalogEntries pins that every diagnostic code declared in
// internal/warn has an entry in both language tables. The drain site renders
// these via Catalog.Msg, so a missing key would surface the raw code string.
func TestWarnCodesHaveCatalogEntries(t *testing.T) {
	t.Parallel()
	codes := warn.Codes()
	if len(codes) == 0 {
		t.Fatal("warn.Codes() returned no codes")
	}
	for _, code := range codes {
		if _, ok := messages[LangEN][code]; !ok {
			t.Errorf("warn code %q missing in en table", code)
		}
		if _, ok := messages[LangJA][code]; !ok {
			t.Errorf("warn code %q missing in ja table", code)
		}
	}
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
