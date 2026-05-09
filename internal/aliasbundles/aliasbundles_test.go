package aliasbundles_test

import (
	"reflect"
	"testing"

	"github.com/sukekyo26/cocoon/internal/aliasbundles"
)

func TestAliasBundleByID(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"git", "ls", "docker"} {
		if got := aliasbundles.AliasBundleByID(id); got == nil || got.ID != id {
			t.Errorf("%q lookup: got %#v", id, got)
		}
	}
	if got := aliasbundles.AliasBundleByID("does-not-exist"); got != nil {
		t.Errorf("expected nil for unknown id, got %#v", got)
	}
}

func TestDefaultAliasBundleIDs(t *testing.T) {
	t.Parallel()

	// All bundles default OFF: aliases are personal preference and should
	// not be baked into a generated workspace.toml.
	got := aliasbundles.DefaultAliasBundleIDs()
	if len(got) != 0 {
		t.Errorf("default ids: got %v, want []", got)
	}
}

func TestExpandAliasBundles(t *testing.T) {
	t.Parallel()

	t.Run("single", func(t *testing.T) {
		t.Parallel()
		got := aliasbundles.ExpandAliasBundles([]string{"git"})
		if got["gs"] != "git status" || got["gl"] != "git log --oneline --graph --decorate" {
			t.Errorf("git bundle: got %v", got)
		}
		if len(got) != 7 {
			t.Errorf("git bundle should have 7 entries, got %d (%v)", len(got), got)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		t.Parallel()
		got := aliasbundles.ExpandAliasBundles([]string{"git", "ls"})
		if got["gs"] != "git status" || got["ll"] != "ls -lah" {
			t.Errorf("merged bundle: got %v", got)
		}
		if len(got) != 7+3 {
			t.Errorf("git+ls should have 10 entries, got %d (%v)", len(got), got)
		}
	})

	t.Run("unknown ignored", func(t *testing.T) {
		t.Parallel()
		got := aliasbundles.ExpandAliasBundles([]string{"git", "does-not-exist"})
		want := aliasbundles.ExpandAliasBundles([]string{"git"})
		if !reflect.DeepEqual(got, want) {
			t.Errorf("unknown id should be skipped: got %v, want %v", got, want)
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		got := aliasbundles.ExpandAliasBundles(nil)
		if len(got) != 0 {
			t.Errorf("nil input should give empty map, got %v", got)
		}
	})
}

// TestAliasBundlesNoKeyCollisions guards against future catalog edits that
// silently make two bundles fight over the same alias key. The whole point
// of bundling-vs-free-input is to keep collisions impossible by curation.
func TestAliasBundlesNoKeyCollisions(t *testing.T) {
	t.Parallel()

	owner := make(map[string]string) // alias key -> bundle id
	for _, b := range aliasbundles.AliasBundles {
		for k := range b.Aliases {
			if prev, dup := owner[k]; dup {
				t.Errorf("alias key %q is defined in both bundle %q and bundle %q", k, prev, b.ID)
			}
			owner[k] = b.ID
		}
	}
}
