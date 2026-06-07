package plugin

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// ErrConflict lets callers detect the category via errors.Is.
var ErrConflict = errors.New("plugin conflict")

// CheckConflicts returns an error when any enabled plugin declares one of
// the other enabled plugins in its metadata.conflicts list. Plugin ids are
// scanned in sorted order and each plugin's conflicts in their declared
// order, so when more than one conflict exists the reported pair is the
// same on every run. A plugin that lists its own id is ignored — a plugin
// cannot conflict with itself.
func CheckConflicts(plugins map[string]*Plugin) error {
	ids := slices.Sorted(maps.Keys(plugins))

	for _, id := range ids {
		p := plugins[id]
		for _, conflictID := range p.Metadata.Conflicts {
			if conflictID == id {
				continue // a plugin listing itself is a no-op, not a conflict
			}
			other, ok := plugins[conflictID]
			if !ok {
				continue
			}
			nameA := p.Metadata.Name
			if nameA == "" {
				nameA = id
			}
			nameB := other.Metadata.Name
			if nameB == "" {
				nameB = conflictID
			}
			return fmt.Errorf(
				"%w: '%s' (%s) conflicts with '%s' (%s); "+
					"disable one of them in your config file's [plugins].enable",
				ErrConflict, nameA, id, nameB, conflictID,
			)
		}
	}
	return nil
}
