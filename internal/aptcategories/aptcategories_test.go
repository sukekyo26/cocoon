package aptcategories_test

import (
	"reflect"
	"testing"

	"github.com/sukekyo26/cocoon/internal/aptcategories"
)

func TestAptCategoryByID(t *testing.T) {
	t.Parallel()

	if got := aptcategories.AptCategoryByID("text-editors"); got == nil || got.ID != "text-editors" {
		t.Fatalf("text-editors lookup failed: %#v", got)
	}
	if got := aptcategories.AptCategoryByID("vcs"); got == nil || got.ID != "vcs" {
		t.Fatalf("vcs lookup failed: %#v", got)
	}
	if got := aptcategories.AptCategoryByID("utilities"); got == nil || got.ID != "utilities" {
		t.Fatalf("utilities lookup failed: %#v", got)
	}
	if got := aptcategories.AptCategoryByID("does-not-exist"); got != nil {
		t.Fatalf("expected nil for unknown id, got %#v", got)
	}
}

func TestDefaultAptCategoryIDs(t *testing.T) {
	t.Parallel()

	got := aptcategories.DefaultAptCategoryIDs()
	want := []string{"text-editors", "vcs", "utilities", "compression", "build"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default ids: got %v, want %v", got, want)
	}
}

func TestExpandAptCategoriesDeduplicates(t *testing.T) {
	t.Parallel()

	got := aptcategories.ExpandAptCategories([]string{"text-editors", "text-editors", "compression"})
	want := []string{"vim", "nano", "zip", "unzip", "xz-utils", "tar", "gzip", "bzip2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expanded packages: got %v, want %v", got, want)
	}
}

func TestExpandAptCategoriesIgnoresUnknown(t *testing.T) {
	t.Parallel()

	got := aptcategories.ExpandAptCategories([]string{"text-editors", "does-not-exist"})
	want := []string{"vim", "nano"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expanded packages: got %v, want %v", got, want)
	}
}

func TestExpandAptCategoriesNewCategories(t *testing.T) {
	t.Parallel()

	// Pin the contents of vcs / utilities so future drift is caught at CI
	// rather than at `apt-get install` time inside a built image.
	got := aptcategories.ExpandAptCategories([]string{"vcs", "utilities"})
	want := []string{
		"git", "openssh-client", "gnupg",
		"tree", "less", "rsync", "file", "bc", "wget", "gettext-base", "uuid-runtime",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("vcs+utilities packages: got %v, want %v", got, want)
	}
}

func TestExpandAptCategoriesDevTools(t *testing.T) {
	t.Parallel()

	// dev-tools is default OFF; pin its package list so a silent reorder /
	// addition is caught at CI rather than at `apt-get install` time.
	got := aptcategories.ExpandAptCategories([]string{"dev-tools"})
	want := []string{"git-lfs", "strace", "tmux"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dev-tools packages: got %v, want %v", got, want)
	}
	// Sanity: dev-tools must NOT be in the default-on set.
	for _, id := range aptcategories.DefaultAptCategoryIDs() {
		if id == "dev-tools" {
			t.Fatal("dev-tools is in DefaultAptCategoryIDs but must be default OFF")
		}
	}
}

func TestExpandAptCategoriesAgent(t *testing.T) {
	t.Parallel()

	// agent is the AI-agent bundle (default OFF); pin its package list and
	// order so a silent change is caught at CI rather than at `apt-get install`
	// time inside a built image.
	got := aptcategories.ExpandAptCategories([]string{"agent"})
	want := []string{
		"jq", "yq", "ripgrep", "fd-find", "tree",
		"python3", "python3-pip", "python3-venv",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("agent packages: got %v, want %v", got, want)
	}
	// Sanity: agent must NOT be in the default-on set (opt-in only).
	for _, id := range aptcategories.DefaultAptCategoryIDs() {
		if id == "agent" {
			t.Fatal("agent is in DefaultAptCategoryIDs but must be default OFF")
		}
	}
}

func TestExpandAptCategoriesAgentDedupesWithNeighbors(t *testing.T) {
	t.Parallel()

	// agent overlaps search (ripgrep, fd-find) and utilities (tree). The doc
	// claim is that selecting agent alongside them is idempotent: each shared
	// package appears exactly once, in first-seen order.
	got := aptcategories.ExpandAptCategories([]string{"agent", "search", "utilities"})
	want := []string{
		"jq", "yq", "ripgrep", "fd-find", "tree",
		"python3", "python3-pip", "python3-venv",
		"fzf", "bat",
		"less", "rsync", "file", "bc", "wget", "gettext-base", "uuid-runtime",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("agent+search+utilities packages: got %v, want %v", got, want)
	}
}
