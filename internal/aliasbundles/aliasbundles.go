// Package aliasbundles defines the curated shell-alias preset groups offered
// by `cocoon init`'s interactive bootstrap. Bundles are presented as a
// multi-select; selected bundles are merged into [container.shell] aliases
// in the generated workspace.toml.
package aliasbundles

import "sort"

// AliasBundle groups a themed set of shell aliases. Description shows the
// alias keys (not values) so the picker stays scannable in one terminal width.
type AliasBundle struct {
	ID          string
	Label       string
	Description string
	Aliases     map[string]string
	// Default pre-checks the box in `cocoon init`'s multi-select. All
	// bundles ship false because shell aliases are personal preference.
	Default bool
}

// AliasBundles is the curated list `cocoon init` shows. Order is the order
// options appear to the user.
//
//nolint:gochecknoglobals // tabular configuration data, file-scoped by design.
var AliasBundles = []AliasBundle{
	{
		ID:          "git",
		Label:       "Git",
		Description: "gs, ga, gc, gp, gco, gd, gl",
		Aliases: map[string]string{
			"gs":  "git status",
			"ga":  "git add",
			"gc":  "git commit",
			"gp":  "git push",
			"gco": "git checkout",
			"gd":  "git diff",
			"gl":  "git log --oneline --graph --decorate",
		},
		Default: false,
	},
	{
		ID:          "ls",
		Label:       "Listing",
		Description: "ll, la, lt",
		Aliases: map[string]string{
			"ll": "ls -lah",
			"la": "ls -A",
			"lt": "ls -ltrh",
		},
		Default: false,
	},
	{
		ID:          "docker",
		Label:       "Docker",
		Description: "d, dc, dcup, dcdown",
		Aliases: map[string]string{
			"d":      "docker",
			"dc":     "docker compose",
			"dcup":   "docker compose up -d",
			"dcdown": "docker compose down",
		},
		Default: false,
	},
}

// AliasBundleByID returns nil when id is not in AliasBundles.
func AliasBundleByID(id string) *AliasBundle {
	for i := range AliasBundles {
		if AliasBundles[i].ID == id {
			return &AliasBundles[i]
		}
	}
	return nil
}

// DefaultAliasBundleIDs preserves catalog order. Currently empty (all
// bundles ship Default=false).
func DefaultAliasBundleIDs() []string {
	var ids []string
	for _, b := range AliasBundles {
		if b.Default {
			ids = append(ids, b.ID)
		}
	}
	return ids
}

// ExpandAliasBundles skips unknown IDs. TestAliasBundlesNoKeyCollisions
// guards against future key collisions in the curated list.
func ExpandAliasBundles(ids []string) map[string]string {
	// Sort the input for deterministic merge order (tests pin the TOML).
	sorted := make([]string, len(ids))
	copy(sorted, ids)
	sort.Strings(sorted)

	out := make(map[string]string)
	for _, id := range sorted {
		b := AliasBundleByID(id)
		if b == nil {
			continue
		}
		for k, v := range b.Aliases {
			out[k] = v
		}
	}
	return out
}
