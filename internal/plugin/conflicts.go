package plugin

import (
	"errors"
	"fmt"
)

// ErrConflict lets callers detect the category via errors.Is.
var ErrConflict = errors.New("plugin conflict")

// CheckConflicts returns an error when any enabled plugin declares one of
// the other enabled plugins in its metadata.conflicts list.
func CheckConflicts(plugins map[string]*Plugin) error {
	for id, p := range plugins {
		for _, conflictID := range p.Metadata.Conflicts {
			if other, ok := plugins[conflictID]; ok {
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
						"disable one of them in workspace.toml [plugins].enable",
					ErrConflict, nameA, id, nameB, conflictID,
				)
			}
		}
	}
	return nil
}
