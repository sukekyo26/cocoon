// Package aliasbundles defines the curated shell-alias preset groups offered
// by `cocoon init`'s interactive bootstrap. Bundles are presented as a
// multi-select; selected bundles are merged into [container.shell] aliases
// in the generated workspace.toml.
package aliasbundles

import "sort"

// AliasBundle groups a small, opinionated set of shell aliases that share
// a theme (git, ls, docker, ...). The ID is the cobra flag value and the
// multi-select option key; Label is what the user sees in the picker;
// Description shows the alias keys (not values) so the picker stays
// scannable in one terminal width.
type AliasBundle struct {
	ID          string
	Label       string
	Description string
	Aliases     map[string]string
	// Default is true when the bundle should start with the box pre-checked
	// in `cocoon init`'s multi-select. All bundles ship Default=false today
	// because shell aliases are personal preference; opt-in is safer than
	// baking opinions into a generated workspace.toml.
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

// AliasBundleByID returns the bundle with the given ID, or nil when the ID
// is not in [AliasBundles].
func AliasBundleByID(id string) *AliasBundle {
	for i := range AliasBundles {
		if AliasBundles[i].ID == id {
			return &AliasBundles[i]
		}
	}
	return nil
}

// DefaultAliasBundleIDs returns the IDs of bundles whose Default flag is
// true, preserving the catalog order. Currently empty: shell aliases are
// personal preference and shouldn't be baked into a generated file.
func DefaultAliasBundleIDs() []string {
	var ids []string
	for _, b := range AliasBundles {
		if b.Default {
			ids = append(ids, b.ID)
		}
	}
	return ids
}

// ExpandAliasBundles returns the merged alias map for the given bundle IDs.
// Unknown IDs are skipped. Bundles that share an alias key would resolve to
// the last winner in iteration order; since the whole point of curating this
// list is to avoid such collisions, [TestAliasBundlesNoKeyCollisions] guards
// against future drift.
func ExpandAliasBundles(ids []string) map[string]string {
	// Sort the input so callers get deterministic iteration when they
	// combine the result with their own alias map (e.g. tests that pin
	// the rendered TOML).
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
