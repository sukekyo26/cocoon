package plugin

import (
	"errors"
	"fmt"
)

// ErrConflict is returned by CheckConflicts when two enabled plugins
// declare each other in metadata.conflicts. Callers can compare with
// errors.Is to detect the category.
var ErrConflict = errors.New("plugin conflict")

// CheckConflicts returns an error when any enabled plugin declares one of
// the other enabled plugins in its metadata.conflicts list. The message
// format mirrors the Python sys.exit(1) branch so existing CLI output stays
// stable.
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
