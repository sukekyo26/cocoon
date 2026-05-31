package aptcategories_test

import (
	"os"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/aptcategories"
)

// e2eAptCategoriesPath is the shared data file docker-roundtrip.sh reads to
// build its --apt-categories set. Relative to this package dir (the go test
// cwd) the repo root is two levels up.
const e2eAptCategoriesPath = "../../e2e/apt-categories.txt"

// readE2EAptCategories parses the shared file the same way
// docker-roundtrip.sh does: one category id per line, skipping blanks and
// #-comments, trimming surrounding space — keep them in lockstep so a stray
// space can't pass this guard yet expand differently at runtime.
func readE2EAptCategories(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(e2eAptCategoriesPath)
	if err != nil {
		t.Fatalf("read %s: %v", e2eAptCategoriesPath, err)
	}
	var ids []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ids = append(ids, line)
	}
	return ids
}

// TestE2EAptCategoriesCoversAll guards that e2e/apt-categories.txt lists every
// category in AptCategories — so the round-trip build actually `apt-get
// install`s every catalog package — and lists nothing stale, since a
// renamed/removed category would otherwise fail the build with an
// unknown-category usage error.
func TestE2EAptCategoriesCoversAll(t *testing.T) {
	t.Parallel()

	fileIDs := readE2EAptCategories(t)
	if len(fileIDs) == 0 {
		t.Fatal("e2e/apt-categories.txt parsed empty — path or format drift")
	}

	inFile := make(map[string]bool, len(fileIDs))
	for _, id := range fileIDs {
		if inFile[id] {
			t.Errorf("duplicate id %q in e2e/apt-categories.txt", id)
		}
		inFile[id] = true
	}

	// Every category must be exercised by the e2e build.
	for _, c := range aptcategories.AptCategories {
		if !inFile[c.ID] {
			t.Errorf("category %q missing from e2e/apt-categories.txt — add it so the round-trip installs its packages", c.ID)
		}
	}

	// No stale entries: every file id must be a real category.
	for _, id := range fileIDs {
		if aptcategories.AptCategoryByID(id) == nil {
			t.Errorf("e2e/apt-categories.txt id %q is not a known apt category", id)
		}
	}
}
