package plugin_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

func TestCheckConflicts(t *testing.T) {
	t.Parallel()
	plugs := map[string]*plugin.Plugin{
		"a": {Metadata: plugin.Metadata{Name: "A", Conflicts: []string{"b"}}},
		"b": {Metadata: plugin.Metadata{Name: "B"}},
	}
	err := plugin.CheckConflicts(plugs)
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	if !strings.Contains(err.Error(), "'A' (a) conflicts with 'B' (b)") {
		t.Errorf("message: %v", err)
	}
	if !errors.Is(err, plugin.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}

	delete(plugs, "b")
	if err := plugin.CheckConflicts(plugs); err != nil {
		t.Errorf("unexpected: %v", err)
	}
}
