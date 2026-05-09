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
	if got := aptcategories.AptCategoryByID("does-not-exist"); got != nil {
		t.Fatalf("expected nil for unknown id, got %#v", got)
	}
}

func TestDefaultAptCategoryIDs(t *testing.T) {
	t.Parallel()

	got := aptcategories.DefaultAptCategoryIDs()
	want := []string{"text-editors", "compression", "build"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default ids: got %v, want %v", got, want)
	}
}

func TestExpandAptCategoriesDeduplicates(t *testing.T) {
	t.Parallel()

	got := aptcategories.ExpandAptCategories([]string{"text-editors", "text-editors", "compression"})
	want := []string{"vim", "nano", "zip", "unzip", "xz-utils"}
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
